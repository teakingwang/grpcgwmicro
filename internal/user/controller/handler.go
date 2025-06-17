package controller

import (
	"context"

	"github.com/teakingwang/grpcgwmicro/api/user"
	"github.com/teakingwang/grpcgwmicro/internal/user/service"
)

type UserController struct {
	user.UnimplementedUserServiceServer
	svc service.UserService
}

func NewUserController(svc service.UserService) *UserController {
	return &UserController{svc: svc}
}

func (uc *UserController) GetUser(ctx context.Context, req *user.GetUserRequest) (*user.GetUserResponse, error) {
	u, err := uc.svc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &user.GetUserResponse{
		Id:       u.ID,
		Username: u.Username,
		Email:    u.Email,
	}, nil
}
