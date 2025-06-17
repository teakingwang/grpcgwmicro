package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/teakingwang/grpcgwmicro/pkg/auth"
)

var noAuthPrefixes = []string{
	"/v1/user/login",
	"/v1/user/signup",
	"/v1/order/",
}

func isNoAuthPath(path string) bool {
	for _, prefix := range noAuthPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// JWTMiddleware 验证 JWT，如果路径在 noAuthPaths 中则跳过
func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isNoAuthPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		token := extractTokenFromHeader(r)
		if token == "" {
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}

		claims, err := auth.ParseToken(token)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "user", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}
