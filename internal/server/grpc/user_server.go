package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	userv1 "storemesh-user-service/gen/user/v1"
	authcontext "storemesh-user-service/internal/auth"
	"storemesh-user-service/internal/domain"
)

type UserGRPCServer struct {
	userv1.UnimplementedUserServiceServer

	service domain.UserService
}

func NewUserGRPCServer(
	service domain.UserService,
) *UserGRPCServer {
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
	if err := requireSelfOrRole(
		ctx,
		req.Id,
		"admin",
	); err != nil {
		return nil, err
	}

	user, err := s.service.GetUser(
		ctx,
		req.Id,
	)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return toProto(user), nil
}

func (s *UserGRPCServer) ListUsers(
	ctx context.Context,
	req *userv1.ListUsersRequest,
) (*userv1.ListUsersResponse, error) {
	if err := requireRole(
		ctx,
		"admin",
	); err != nil {
		return nil, err
	}

	result, err := s.service.ListUsers(
		ctx,
		domain.ListUsersRequest{
			Page:    int(req.Page),
			PerPage: int(req.PerPage),
			Status: domain.UserStatus(
				req.Status,
			),
		},
	)
	if err != nil {
		return nil, toGRPCError(err)
	}

	users := make(
		[]*userv1.User,
		0,
		len(result.Users),
	)

	for _, user := range result.Users {
		users = append(
			users,
			toProto(user),
		)
	}

	return &userv1.ListUsersResponse{
		Users:      users,
		TotalItems: result.TotalItems,
		TotalPages: int32(result.TotalPages),
		Page:       int32(result.Page),
		PerPage:    int32(result.PerPage),
	}, nil
}

func (s *UserGRPCServer) DeleteUser(
	ctx context.Context,
	req *userv1.DeleteUserRequest,
) (*userv1.DeleteUserResponse, error) {
	if err := requireSelfOrRole(
		ctx,
		req.Id,
		"admin",
	); err != nil {
		return nil, err
	}

	if err := s.service.DeleteUser(
		ctx,
		req.Id,
	); err != nil {
		return nil, toGRPCError(err)
	}

	return &userv1.DeleteUserResponse{
		Success: true,
	}, nil
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

func requireSelfOrRole(
	ctx context.Context,
	requestedUserID string,
	roles ...string,
) error {
	claims, err := authcontext.RequireClaims(ctx)
	if err != nil {
		return status.Error(
			codes.Unauthenticated,
			"authentication required",
		)
	}

	if claims.UserID == requestedUserID {
		return nil
	}

	if authcontext.HasAnyRole(
		claims,
		roles...,
	) {
		return nil
	}

	return status.Error(
		codes.PermissionDenied,
		"forbidden",
	)
}

func requireRole(
	ctx context.Context,
	roles ...string,
) error {
	claims, err := authcontext.RequireClaims(ctx)
	if err != nil {
		return status.Error(
			codes.Unauthenticated,
			"authentication required",
		)
	}

	if authcontext.HasAnyRole(
		claims,
		roles...,
	) {
		return nil
	}

	return status.Error(
		codes.PermissionDenied,
		"forbidden",
	)
}

func toProto(
	user *domain.User,
) *userv1.User {
	if user == nil {
		return nil
	}

	return &userv1.User{
		Id:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Phone:     user.Phone,
		IsActive:  user.IsActive(),
	}
}

func toGRPCError(
	err error,
) error {
	switch {
	case errors.Is(
		err,
		domain.ErrNotFound,
	):
		return status.Error(
			codes.NotFound,
			err.Error(),
		)

	case errors.Is(
		err,
		domain.ErrAlreadyExists,
	):
		return status.Error(
			codes.AlreadyExists,
			err.Error(),
		)

	case errors.Is(
		err,
		domain.ErrInvalidPassword,
	):
		return status.Error(
			codes.Unauthenticated,
			err.Error(),
		)

	case errors.Is(
		err,
		domain.ErrInvalidToken,
	):
		return status.Error(
			codes.Unauthenticated,
			err.Error(),
		)

	case errors.Is(
		err,
		domain.ErrUnauthorized,
	):
		return status.Error(
			codes.Unauthenticated,
			err.Error(),
		)

	case errors.Is(
		err,
		domain.ErrForbidden,
	):
		return status.Error(
			codes.PermissionDenied,
			err.Error(),
		)

	case errors.Is(
		err,
		domain.ErrInvalidInput,
	):
		return status.Error(
			codes.InvalidArgument,
			err.Error(),
		)

	default:
		return status.Error(
			codes.Internal,
			"internal server error",
		)
	}
}
