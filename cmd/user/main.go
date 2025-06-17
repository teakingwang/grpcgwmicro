package main

import (
	"fmt"
	"github.com/teakingwang/grpcgwmicro/config"
	"github.com/teakingwang/grpcgwmicro/internal/user/app"
	"github.com/teakingwang/grpcgwmicro/pkg/consul"
	"github.com/teakingwang/grpcgwmicro/pkg/logger"
	"github.com/teakingwang/grpcgwmicro/pkg/utils/idgen"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"github.com/teakingwang/grpcgwmicro/api/user"
	"github.com/teakingwang/grpcgwmicro/internal/user/controller"
	"google.golang.org/grpc"
)

func main() {
	if err := logger.Init(true); err != nil {
		panic("logger init failed: " + err.Error())
	}

	// 尝试从 Consul 加载，如果失败则回退到本地 config.yaml
	if err := config.LoadConfigFromConsul("config/user"); err != nil {
		logger.Warn("load from consul failed: %v, falling back to local config", err)
		if err := config.LoadConfig(); err != nil {
			panic(fmt.Errorf("failed to load config: %v", err))
		}
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("panic occurred: %v", r)
			logger.Errorf("stack trace:\n%s", string(debug.Stack()))
		}
		logger.Sync() // 🟢 放在这里，保证所有日志都 flush
	}()

	if err := run(); err != nil {
		logger.Errorf("service exited with error: %v", err)
	}
}

func run() error {
	lis, err := net.Listen("tcp", ":"+config.Config.Server.User.Port)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	// 初始化 ID 生成器
	if err := idgen.Init(); err != nil {
		return fmt.Errorf("failed to initialize idgen: %v", err)
	}

	ctx, err := app.NewAppContext()
	if err != nil {
		return fmt.Errorf("new appcontext err:%v", err)
	}

	s := grpc.NewServer()
	registerHealthCheck(s) // 👈 注册 gRPC 健康检查
	// --- 集成 Consul 服务发现 ---
	consulClient, err := consul.NewConsulClient(config.Config.Consul.Address)
	if err != nil {
		return fmt.Errorf("failed to create consul client: %v", err)
	}

	serviceID := config.GetServiceID()
	serviceName := config.GetServiceName()
	serviceAddress := config.GetServiceAddress()
	servicePort, err := strconv.Atoi(config.Config.Server.User.Port) // 注意错误处理
	if err != nil {
		return fmt.Errorf("invalid service port: %v", err)
	}

	logger.Info("Registering service to consul", serviceID, serviceName, serviceAddress, servicePort)
	if err := consulClient.RegisterService(serviceID, serviceName, serviceAddress, servicePort, []string{"grpc", "user"}); err != nil {
		return fmt.Errorf("failed to register service to consul: %v", err)
	}
	logger.Infof("Service registered to consul: %s", serviceID)

	user.RegisterUserServiceServer(s, controller.NewUserController(ctx.UserService))

	// 监听退出信号，优雅注销服务
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stopCh
		logger.Info("Shutdown signal received, deregistering service...")
		if err := consulClient.DeregisterService(serviceID); err != nil {
			logger.Errorf("failed to deregister service: %v", err)
		} else {
			logger.Info("Service deregistered from consul")
		}
		logger.Sync()
		time.Sleep(time.Second) // 确保注销完成
		os.Exit(0)
	}()

	log.Println("user-service gRPC server started on :50051")
	if err := s.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}

	return nil
}

func registerHealthCheck(s *grpc.Server) {
	hs := health.NewServer()
	hs.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s, hs)
}
