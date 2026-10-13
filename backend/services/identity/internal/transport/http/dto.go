package http

import (
	"time"

	"github.com/Furkanturan8/rovena/services/identity/internal/application"
	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
)

type RegisterRequest struct {
	Email     string `json:"email" validate:"required,email,max=254"`
	Password  string `json:"password" validate:"required,min=8,max=72"`
	FirstName string `json:"first_name" validate:"max=100"`
	LastName  string `json:"last_name" validate:"max=100"`
}

func (r RegisterRequest) ToInput() application.RegisterInput {
	return application.RegisterInput{
		Email: r.Email, Password: r.Password, FirstName: r.FirstName, LastName: r.LastName,
	}
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type UpdateProfileRequest struct {
	FirstName *string `json:"first_name" validate:"omitempty,max=100"`
	LastName  *string `json:"last_name" validate:"omitempty,max=100"`
}

type ChangeRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=customer admin"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func ToUserResponse(u *domain.User) UserResponse {
	return UserResponse{
		ID: u.ID.String(), Email: u.Email, FirstName: u.FirstName, LastName: u.LastName,
		Role: string(u.Role), CreatedAt: u.CreatedAt,
	}
}

type TokensResponse struct {
	AccessToken      string    `json:"access_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	TokenType        string    `json:"token_type"`
}

func ToTokensResponse(t application.TokenPair) TokensResponse {
	return TokensResponse{
		AccessToken: t.AccessToken, AccessExpiresAt: t.AccessExpiresAt,
		RefreshToken: t.RefreshToken, RefreshExpiresAt: t.RefreshExpiresAt,
		TokenType: "Bearer",
	}
}

type AuthResponse struct {
	User   UserResponse   `json:"user"`
	Tokens TokensResponse `json:"tokens"`
}

func ToAuthResponse(r *application.AuthResult) AuthResponse {
	return AuthResponse{User: ToUserResponse(r.User), Tokens: ToTokensResponse(r.Tokens)}
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=72"`
}

type ChangeEmailRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required"`
}

type CreateAddressRequest struct {
	Title      string `json:"title" validate:"required,max=40"`
	FullName   string `json:"full_name" validate:"required,max=100"`
	Phone      string `json:"phone" validate:"required,min=7,max=24"`
	Line1      string `json:"line1" validate:"required,max=200"`
	Line2      string `json:"line2" validate:"max=200"`
	District   string `json:"district" validate:"required,max=80"`
	City       string `json:"city" validate:"required,max=80"`
	PostalCode string `json:"postal_code" validate:"max=16"`
	IsDefault  bool   `json:"is_default"`
}

type UpdateAddressRequest struct {
	Title      *string `json:"title" validate:"omitempty,min=1,max=40"`
	FullName   *string `json:"full_name" validate:"omitempty,min=1,max=100"`
	Phone      *string `json:"phone" validate:"omitempty,min=7,max=24"`
	Line1      *string `json:"line1" validate:"omitempty,min=1,max=200"`
	Line2      *string `json:"line2" validate:"omitempty,max=200"`
	District   *string `json:"district" validate:"omitempty,min=1,max=80"`
	City       *string `json:"city" validate:"omitempty,min=1,max=80"`
	PostalCode *string `json:"postal_code" validate:"omitempty,max=16"`
	IsDefault  *bool   `json:"is_default"`
}

type AddressResponse struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	FullName   string    `json:"full_name"`
	Phone      string    `json:"phone"`
	Line1      string    `json:"line1"`
	Line2      string    `json:"line2"`
	District   string    `json:"district"`
	City       string    `json:"city"`
	PostalCode string    `json:"postal_code"`
	Country    string    `json:"country"`
	IsDefault  bool      `json:"is_default"`
	CreatedAt  time.Time `json:"created_at"`
}

func ToAddressResponse(a *domain.Address) AddressResponse {
	return AddressResponse{
		ID: a.ID.String(), Title: a.Title, FullName: a.FullName, Phone: a.Phone, Line1: a.Line1, Line2: a.Line2,
		District: a.District, City: a.City, PostalCode: a.PostalCode, Country: a.Country,
		IsDefault: a.IsDefault, CreatedAt: a.CreatedAt,
	}
}
