package user

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"

	"github.com/sabbatD/srest-api/internal/lib/api/access"
	resp "github.com/sabbatD/srest-api/internal/lib/api/response"
	"github.com/sabbatD/srest-api/internal/lib/logger/sl"
	u "github.com/sabbatD/srest-api/internal/lib/userConfig"
)

type AuthResponse struct {
	resp.Response
	Token string `json:"token,omitempty"`
}

type RegisterResponse struct {
	resp.Response
	Authdata u.AuthData `json:"authdata,omitempty"`
}

type GetResponse struct {
	resp.Response
	User u.TableUser `json:"user,omitempty"`
}

type UserHandler interface {
	Add(u u.User) (int64, error)
	Auth(u u.AuthData) (succsess, isAdmin bool, id int, err error)
	Get(id int) (u.TableUser, error)
	UpdateUser(u u.User, id int) (int64, error)
}

// Register godoc
// @Summary Register a new user
// @Description Handles the registration of a new user by accepting a JSON payload containing user data.
// @Tags user
// @Accept json
// @Produce json
// @Param UserData body u.User true "Complete user data for registration"
// @Success 200 {object} RegisterResponse "Registration successful. Returns user authentication data."
// @Failure 401 {object} resp.Response "Registration failed. Returns error message."
// @Router /signup [post]
func Register(log *slog.Logger, user UserHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.hanlders.user.Register"

		log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req u.User
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			JsonDecodeError(w, r, log, err)
			return
		}

		log.Info("request body decoded", slog.Any("request", req))

		_, err := user.Add(req)
		if err != nil {
			if err.Error() == "database.postgres.Add: user already exists" {
				log.Info(err.Error())

				render.JSON(w, r, resp.Error("user already exists"))

				return
			}
			InternalError(w, r, log, err)
			return
		}

		log.Info("user successfully created")
		render.JSON(w, r, RegisterResponse{resp.OK(), u.AuthData{Login: req.Login, Password: req.Password}})
	}
}

// Auth handles user authentication.
// @Summary Authenticate user
// @Description This endpoint authenticates a user by accepting their login credentials.
// @Tags user
// @Accept json
// @Produce json
// @Param AuthData body u.AuthData true "User login credentials"
// @Success 200 {object} AuthResponse "Returns a token if authentication succeeds."
// @Failure 401 {object} resp.Response "Authentication failed. Returns error message."
// @Router /signin [post]
func Auth(log *slog.Logger, user UserHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.hanlders.user.Auth"

		log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req u.AuthData
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			JsonDecodeError(w, r, log, err)
			return
		}

		log.Info("request body decoded", slog.Any("request", req))

		ok, isAdmin, id, err := user.Auth(req)
		if err != nil {
			if !ok {
				log.Info(err.Error())

				render.JSON(w, r, resp.Error("wrong login or password"))

				return
			}
			InternalError(w, r, log, err)
			return
		}

		token, err := access.GenerateJWT(id, req.Login, isAdmin)
		if err != nil {
			InternalError(w, r, log, fmt.Errorf("could not generate JWT Token"))
			return
		}

		log.Info("successfully logged in")
		render.JSON(w, r, AuthResponse{resp.OK(), token})
	}
}

// UpdateUser handles user profile updates.
// @Summary Update user profile
// @Description Updates the entire user profile with the new data provided.
// @Tags user
// @Accept json
// @Produce json
// @Param Userdata body u.User true "Updated user data"
// @Success 200 {object} resp.Response "Returns success if the update was successful."
// @Failure 401 {object} resp.Response "Update failed. Returns error message."
// @Router /user/profile [put]
func UpdateUser(log *slog.Logger, user UserHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.hanlders.user.UpdateUser"

		log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req u.User
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			JsonDecodeError(w, r, log, err)
			return
		}

		log.Info("request body decoded", slog.Any("request", req))

		userContext, ok := r.Context().Value(access.CxtKey("userContext")).(access.UserContext)
		if !ok {
			http.Error(w, "User context not found", http.StatusUnauthorized)
			return
		}

		n, err := user.UpdateUser(req, userContext.Id)
		if err != nil {
			if n == 0 {
				log.Info(err.Error())

				render.JSON(w, r, resp.Error(err.Error()))
			}
			InternalError(w, r, log, err)
			return
		}

		log.Info(fmt.Sprintf("Successfully updated user: %v to %v with password %v and email %v", userContext.Id, req.Username, req.Password, req.Email))

		render.JSON(w, r, resp.OK())
	}

}

// Profile returns the user profile data.
// @Summary Get user profile
// @Description Retrieves the full user profile data.
// @Tags user
// @Produce json
// @Success 200 {object} GetResponse "Returns the user profile data."
// @Failure 401 {object} resp.Response "Get profile failed. Returns error message."
// @Router /user/profile [get]
func Profile(log *slog.Logger, user UserHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.hanlders.user.Profile"

		log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		userContext, ok := r.Context().Value(access.CxtKey("userContext")).(access.UserContext)
		if !ok {
			http.Error(w, "User context not found", http.StatusUnauthorized)
			return
		}

		user, err := user.Get(userContext.Id)
		if err != nil {
			if err.Error() == "database.postgres.Get: no such user" {
				log.Info(err.Error())

				render.JSON(w, r, resp.Error("No such user"))

				return
			}
			InternalError(w, r, log, err)
			return
		}

		log.Info("user successfully retrieved")
		render.JSON(w, r, GetResponse{resp.OK(), user})
	}
}

func InternalError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	log.Debug(err.Error())

	render.JSON(w, r, resp.Error("Internal Server Error"))
}

func JsonDecodeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	log.Error("failed to decode request body", sl.Err(err))

	render.JSON(w, r, resp.Error("failed to decode request"))
}
