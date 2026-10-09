package application

import (
	"context"
	"strings"
	"time"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/Furkanturan8/rovena/services/catalog/internal/repository"
	"github.com/google/uuid"
)

type CategoryService struct {
	categories repository.CategoryRepository
}

func NewCategoryService(categories repository.CategoryRepository) *CategoryService {
	return &CategoryService{categories: categories}
}

type CreateCategoryInput struct {
	Name        string
	Description string
	ParentID    *uuid.UUID
}

func (s *CategoryService) Create(ctx context.Context, in CreateCategoryInput) (*domain.Category, error) {
	name := strings.TrimSpace(in.Name)
	slug, err := uniqueSlug(ctx, name, s.categories.SlugExists)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	c := &domain.Category{
		ID: id, ParentID: in.ParentID, Name: name, Slug: slug,
		Description: strings.TrimSpace(in.Description), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.categories.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

type UpdateCategoryInput struct {
	Name        *string
	Description *string
}

// Update never changes the slug or the parent, so URLs stay stable and the tree stays acyclic.
func (s *CategoryService) Update(ctx context.Context, id uuid.UUID, in UpdateCategoryInput) (*domain.Category, error) {
	c, err := s.categories.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		c.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		c.Description = strings.TrimSpace(*in.Description)
	}
	if err := s.categories.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *CategoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.categories.Delete(ctx, id)
}

func (s *CategoryService) List(ctx context.Context) ([]domain.Category, error) {
	return s.categories.List(ctx)
}

func (s *CategoryService) GetBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	return s.categories.GetBySlug(ctx, slug)
}
