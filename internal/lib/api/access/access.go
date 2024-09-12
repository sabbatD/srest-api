package access

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

var jwtKey = []byte("YouAreKissedSabbatsAss")

type CxtKey string

type Claims struct {
	Id      int    `json:"id"`
	Login   string `json:"login"`
	IsAdmin bool   `json:"isadmin"`
	jwt.StandardClaims
}

type UserContext struct {
	Id        int    `json:"id"`
	Login     string `json:"login"`
	IsAdmin   bool   `json:"isadmin"`
	IsBlocked bool   `json:"isblocked"`
}

func NewAccessToken(id int, login string, admin bool) (string, error) {
	expirationTime := time.Now().Add(2 * time.Hour)
	claims := &Claims{
		Id:      id,
		Login:   login,
		IsAdmin: admin,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func NewRefreshToken() string {
	token := jwt.New(jwt.SigningMethodHS256)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return ""
	}

	return tokenString
}

func JWTAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// cookie, err := r.Cookie("token")
		// if err != nil {
		// 	if err == http.ErrNoCookie {
		// 		http.Error(w, "Authorization cookie is missing", http.StatusUnauthorized)
		// 		return
		// 	}
		// 	http.Error(w, err.Error(), http.StatusBadRequest)
		// 	return
		// }
		// tokenString := cookie.Value // раскоммент

		tokenString := r.Header.Get("Authorization") // коммент

		tokenString = strings.TrimPrefix(tokenString, "Bearer ") // коммент

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		userContext := UserContext{
			Id:      claims.Id,
			Login:   claims.Login,
			IsAdmin: claims.IsAdmin,
		}
		ctx := context.WithValue(r.Context(), CxtKey("userContext"), userContext)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
