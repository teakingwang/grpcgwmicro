package main

import (
	"context"
	"fmt"
	"github.com/teakingwang/grpcgwmicro/config"
	"github.com/teakingwang/grpcgwmicro/pkg/auth"
	"github.com/teakingwang/grpcgwmicro/pkg/logger"
	"github.com/teakingwang/grpcgwmicro/pkg/middleware"
	"log"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	"github.com/teakingwang/grpcgwmicro/api/order"
	"github.com/teakingwang/grpcgwmicro/api/user"
)

func main() {
	if err := logger.Init(true); err != nil {
		panic("logger init failed: " + err.Error())
	}

	// 尝试从 Consul 加载，如果失败则回退到本地 config.yaml
	if err := config.LoadConfigFromConsul("config/order"); err != nil {
		logger.Warn("load from consul failed: %v, falling back to local config", err)
		if err := config.LoadConfig(); err != nil {
			panic(fmt.Errorf("failed to load config: %v", err))
		}
	}

	// token 认证相关配置
	auth.Init(config.Config.JWT.Secret, config.Config.JWT.ExpireSeconds)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, middleware.NewWrappedMarshaler()),
		runtime.WithErrorHandler(middleware.CustomErrorInterceptor),
	)
	opts := []grpc.DialOption{grpc.WithInsecure()}

	if err := user.RegisterUserServiceHandlerFromEndpoint(ctx, mux, config.Config.Server.User.Name+":"+config.Config.Server.User.Port, opts); err != nil {
		panic(fmt.Sprintf("failed to register user-service: %v", err))
	}

	if err := order.RegisterOrderServiceHandlerFromEndpoint(ctx, mux, config.Config.Server.Order.Name+":"+config.Config.Server.Order.Port, opts); err != nil {
		panic(fmt.Sprintf("failed to register order-service: %v", err))
	}

	// 包装中间件（JWT 认证 + 响应封装）
	log.Println("🌐 Gateway listening on :8080")
	handler := middleware.JWTMiddleware(mux)
	if err := http.ListenAndServe(":8080", handler); err != nil {
		panic(fmt.Sprintf("failed to serve: %v", err))
	}
}
