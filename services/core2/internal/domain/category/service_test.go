package category_test

import (
	"context"
	. "planeo/services/core2/internal/domain/category"
	"planeo/services/core2/internal/domain/category/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCategoryService(t *testing.T) {
	if !testing.Short() {
		t.Skip()
	}

	testOrganizationId := 1
	categoryCreateInput := NewCategory{
		Label:            "New Category",
		LabelDescription: "A new category description",
		Color:            "#000000",
		OrganizationId:   testOrganizationId,
	}
	categoryUpdateInput := UpdateCategory{
		Id:               1,
		Label:            "Updated Category",
		LabelDescription: "An updated category description",
		Color:            "#FFFFFF",
		OrganizationId:   testOrganizationId,
	}
	category := Category{
		Label:            categoryCreateInput.Label,
		LabelDescription: categoryCreateInput.LabelDescription,
		Color:            categoryCreateInput.Color,
		OrganizationId:   testOrganizationId,
		Id:               1,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	t.Run("CreateCategory", func(t *testing.T) {
		t.Run("returns nil when category is created successfully", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepository(t)
			mockCategoryRepository.EXPECT().CreateCategory(context.Background(), testOrganizationId, categoryCreateInput).Return(1, nil)
			categoryService := NewService(mockCategoryRepository)

			id, err := categoryService.CreateCategory(context.Background(), testOrganizationId, categoryCreateInput)
			assert.Nil(t, err)
			assert.Equal(t, 1, id)
		})

		t.Run("returns error when category creation fails", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepository(t)
			mockCategoryRepository.EXPECT().CreateCategory(context.Background(), testOrganizationId, categoryCreateInput).Return(0, assert.AnError)
			categoryService := NewService(mockCategoryRepository)

			id, err := categoryService.CreateCategory(context.Background(), testOrganizationId, categoryCreateInput)
			assert.Error(t, err)
			assert.Equal(t, 0, id)
		})
	})

	t.Run("UpdateCategory", func(t *testing.T) {
		t.Run("returns nil when category is updated successfully", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepository(t)
			mockCategoryRepository.EXPECT().UpdateCategory(context.Background(), testOrganizationId, category.Id, categoryUpdateInput).Return(nil)
			categoryService := NewService(mockCategoryRepository)

			err := categoryService.UpdateCategory(context.Background(), testOrganizationId, category.Id, categoryUpdateInput)
			assert.Nil(t, err)
		})

		t.Run("returns error when category update fails", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepository(t)
			mockCategoryRepository.EXPECT().UpdateCategory(context.Background(), testOrganizationId, category.Id, categoryUpdateInput).Return(assert.AnError)
			categoryService := NewService(mockCategoryRepository)

			err := categoryService.UpdateCategory(context.Background(), testOrganizationId, category.Id, categoryUpdateInput)
			assert.Error(t, err)
		})
	})

	t.Run("DeleteCategory", func(t *testing.T) {
		t.Run("returns nil when category is deleted successfully", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepository(t)
			mockCategoryRepository.EXPECT().DeleteCategory(context.Background(), testOrganizationId, category.Id).Return(nil)
			categoryService := NewService(mockCategoryRepository)

			err := categoryService.DeleteCategory(context.Background(), testOrganizationId, category.Id)
			assert.Nil(t, err)
		})

		t.Run("returns error when category deletion fails", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepository(t)
			mockCategoryRepository.EXPECT().DeleteCategory(context.Background(), testOrganizationId, category.Id).Return(assert.AnError)
			categoryService := NewService(mockCategoryRepository)

			err := categoryService.DeleteCategory(context.Background(), testOrganizationId, category.Id)
			assert.Error(t, err)
		})
	})

	t.Run("GetCategories", func(t *testing.T) {
		t.Run("returns categories when categories are fetched successfully", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepository(t)
			mockCategoryRepository.EXPECT().GetCategories(context.Background(), testOrganizationId).Return([]Category{category}, nil)
			categoryService := NewService(mockCategoryRepository)

			categories, err := categoryService.GetCategories(context.Background(), testOrganizationId)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(categories))
		})

		t.Run("returns error when categories fetch fails", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepository(t)
			mockCategoryRepository.EXPECT().GetCategories(context.Background(), testOrganizationId).Return(nil, assert.AnError)
			categoryService := NewService(mockCategoryRepository)

			categories, err := categoryService.GetCategories(context.Background(), testOrganizationId)
			assert.Error(t, err)
			assert.Nil(t, categories)
		})
	})
}
