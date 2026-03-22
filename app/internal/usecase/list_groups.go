package usecase

import (
	"context"

	"github.com/solock/solock/internal/domain"
)

type ListGroupsUseCase struct {
	groups domain.GroupRepository
}

func NewListGroupsUseCase(groups domain.GroupRepository) *ListGroupsUseCase {
	return &ListGroupsUseCase{groups: groups}
}

func (uc *ListGroupsUseCase) Execute(ctx context.Context) ([]*domain.Group, error) {
	return uc.groups.List(ctx)
}
