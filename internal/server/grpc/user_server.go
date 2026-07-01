package grpc

import (
	"context"
	"errors"
	"storemesh-user-service/internal/models"

	userv1 "storemesh-user-service/gen/user/v1"
	"storemesh-user-service/internal/domain"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserGRPCServer struct {
	userv1.UnimplementedUserServiceServer

	service domain.UserService
}

func NewUserGRPCServer(service domain.UserService) *UserGRPCServer {
	return &UserGRPCServer{
		service: service,
	}
}

func (s *UserGRPCServer) CreateUser(
	ctx context.Context,
	req *userv1.CreateUserRequest,
) (*userv1.User, error) {

	user, err := s.service.CreateUser(
		ctx,
		domain.CreateUserRequest{
			Email:     req.Email,
			Password:  req.Password,
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Phone:     req.Phone,
		},
	)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return toProto(user), nil
}

func (s *UserGRPCServer) GetUser(
	ctx context.Context,
	req *userv1.GetUserRequest,
) (*userv1.User, error) {

	user, err := s.service.GetUser(ctx, req.Id)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return toProto(user), nil
}

func (s *UserGRPCServer) Authenticate(
	ctx context.Context,
	req *userv1.AuthRequest,
) (*userv1.AuthResponse, error) {

	user, tokens, err := s.service.Authenticate(
		ctx,
		domain.AuthRequest{
			Email:    req.Email,
			Password: req.Password,
		},
	)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &userv1.AuthResponse{
		User:         toProto(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}

// -----------------------------------------------------------------------------
// Proto mappers
// -----------------------------------------------------------------------------

func toProto(u *models.User) *userv1.User {
	return &userv1.User{
		Id:        u.ID.String(),
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Phone:     u.Phone,
	}
}

// -----------------------------------------------------------------------------
// Error mapping
// -----------------------------------------------------------------------------

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, domain.ErrInvalidPassword):
		return status.Error(codes.Unauthenticated, err.Error())

	case errors.Is(err, domain.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, err.Error())

	case errors.Is(err, domain.ErrUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())

	case errors.Is(err, domain.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())

	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())

	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
