package application

import (
	"context"
	"strings"

	"github.com/Furkanturan8/rovena/services/identity/internal/domain"
	"github.com/Furkanturan8/rovena/services/identity/internal/repository"
	"github.com/google/uuid"
)

type UserService struct {
	users repository.UserRepository
}

func NewUserService(users repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

type UpdateProfileInput struct {
	FirstName *string
	LastName  *string
}

func (s *UserService) UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateProfileInput) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.FirstName != nil {
		user.FirstName = strings.TrimSpace(*in.FirstName)
	}
	if in.LastName != nil {
		user.LastName = strings.TrimSpace(*in.LastName)
	}
	if err := s.users.UpdateProfile(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) List(ctx context.Context, p repository.ListUsersParams) ([]domain.User, int, error) {
	return s.users.List(ctx, p)
}

// ChangeRole sets a user's role. The repository guarantees the last admin cannot be
// demoted, even when several admins change roles at the same time.
func (s *UserService) ChangeRole(ctx context.Context, id uuid.UUID, role domain.Role) (*domain.User, error) {
	if !role.Valid() {
		return nil, domain.ErrInvalidRole
	}
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.users.UpdateRole(ctx, id, role); err != nil {
		return nil, err
	}
	user.Role = role
	return user, nil
}
