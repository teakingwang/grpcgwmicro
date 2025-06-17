package main

import (
	"context"
	"fmt"
	"github.com/teakingwang/grpcgwmicro/api/order"
	"github.com/teakingwang/grpcgwmicro/config"
	"github.com/teakingwang/grpcgwmicro/internal/order/app"
	"github.com/teakingwang/grpcgwmicro/internal/order/controller"
	"github.com/teakingwang/grpcgwmicro/internal/task"
	"github.com/teakingwang/grpcgwmicro/pkg/consul"
	"github.com/teakingwang/grpcgwmicro/pkg/logger"
	"github.com/teakingwang/grpcgwmicro/pkg/utils/idgen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"log"
	"net"
	"runtime/debug"
	"strconv"
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

	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("panic occurred: %v", r)
			logger.Errorf("stack trace:\n%s", string(debug.Stack()))
		}
		logger.Sync() //  放在这里，保证所有日志都 flush
	}()

	if err := run(); err != nil {
		logger.Errorf("service exited with error: %v", err)
	}
}

func run() error {
	lis, err := net.Listen("tcp", ":"+config.Config.Server.Order.Port)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	// idgen
	// 初始化 ID 生成器
	if err := idgen.Init(); err != nil {
		return fmt.Errorf("failed to initialize idgen: %v", err)
	}

	ctx, err := app.NewAppContext()
	if err != nil {
		return fmt.Errorf("new appcontext err:%v", err)
	}
	defer ctx.UserConn.Close()

	// 3. 启动 Kafka 消费者协程
	orderConsumer := task.NewOrderConsumer(ctx.KafkaClient, ctx.OrderService)
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := orderConsumer.Run(runCtx); err != nil {
			log.Printf("order consumer stopped: %v", err)
		}
	}()

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
	servicePort, err := strconv.Atoi(config.Config.Server.Order.Port) // 注意错误处理
	if err != nil {
		return fmt.Errorf("invalid service port: %v", err)
	}

	if err := consulClient.RegisterService(serviceID, serviceName, serviceAddress, servicePort, []string{"grpc", "order"}); err != nil {
		return fmt.Errorf("failed to register service to consul: %v", err)
	}
	logger.Infof("Service registered to consul: %s", serviceID)
	order.RegisterOrderServiceServer(s, controller.NewOrderController(ctx.OrderService))

	log.Println("order-service gRPC server started on :50052")
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
