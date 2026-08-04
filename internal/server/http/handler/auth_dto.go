package handler

import "storemesh-user-service/internal/domain"

type authenticateRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (
	r authenticateRequest,
) toDomain() domain.AuthRequest {
	return domain.AuthRequest{
		Email:    r.Email,
		Password: r.Password,
	}
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type tokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

func toTokenPairResponse(
	pair *domain.TokenPair,
) *tokenPairResponse {
	if pair == nil {
		return nil
	}

	return &tokenPairResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    "Bearer",
	}
}

type authenticationResponse struct {
	User   *userResponse      `json:"user"`
	Tokens *tokenPairResponse `json:"tokens"`
}
