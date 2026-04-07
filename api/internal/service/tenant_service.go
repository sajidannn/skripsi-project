package service

import (
	"context"

	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
)

type TenantService struct {
	repo repository.TenantRepository
}

func NewTenantService(repo repository.TenantRepository) *TenantService {
	return &TenantService{repo: repo}
}

func (s *TenantService) ListTenants(ctx context.Context) ([]model.Tenant, error) {
	return s.repo.List(ctx)
}
