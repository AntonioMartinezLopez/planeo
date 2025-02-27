package category

import (
	"context"
	"planeo/api/internal/resources/category/dto"
	"planeo/api/internal/resources/category/mocks"
	"planeo/api/internal/resources/category/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCategoryService(t *testing.T) {

	if !testing.Short() {
		t.Skip()
	}

	testOrganizationId := 1
	categoryCreateInput := dto.CreateCategoryInputBody{
		Label:            "New Category",
		LabelDescription: "A new category description",
		Color:            "#000000",
	}
	categoryUpdateInput := dto.UpdateCategoryInputBody{
		Label:            "Updated Category",
		LabelDescription: "An updated category description",
		Color:            "#FFFFFF",
	}
	category := models.Category{
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
			mockCategoryRepository := mocks.NewMockCategoryRepositoryInterface(t)
			mockCategoryRepository.EXPECT().CreateCategory(context.Background(), testOrganizationId, categoryCreateInput).Return(nil)
			categoryService := NewCategoryService(mockCategoryRepository)

			err := categoryService.CreateCategory(context.Background(), testOrganizationId, categoryCreateInput)
			assert.Nil(t, err)
		})

		t.Run("returns error when category creation fails", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepositoryInterface(t)
			mockCategoryRepository.EXPECT().CreateCategory(context.Background(), testOrganizationId, categoryCreateInput).Return(assert.AnError)
			categoryService := NewCategoryService(mockCategoryRepository)

			err := categoryService.CreateCategory(context.Background(), testOrganizationId, categoryCreateInput)
			assert.Error(t, err)
		})
	})

	t.Run("UpdateCategory", func(t *testing.T) {

		t.Run("returns nil when category is updated successfully", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepositoryInterface(t)
			mockCategoryRepository.EXPECT().UpdateCategory(context.Background(), testOrganizationId, category.Id, categoryUpdateInput).Return(nil)
			categoryService := NewCategoryService(mockCategoryRepository)

			err := categoryService.UpdateCategory(context.Background(), testOrganizationId, category.Id, categoryUpdateInput)
			assert.Nil(t, err)
		})

		t.Run("returns error when category update fails", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepositoryInterface(t)
			mockCategoryRepository.EXPECT().UpdateCategory(context.Background(), testOrganizationId, category.Id, categoryUpdateInput).Return(assert.AnError)
			categoryService := NewCategoryService(mockCategoryRepository)

			err := categoryService.UpdateCategory(context.Background(), testOrganizationId, category.Id, categoryUpdateInput)
			assert.Error(t, err)
		})
	})

	t.Run("DeleteCategory", func(t *testing.T) {

		t.Run("returns nil when category is deleted successfully", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepositoryInterface(t)
			mockCategoryRepository.EXPECT().DeleteCategory(context.Background(), testOrganizationId, category.Id).Return(nil)
			categoryService := NewCategoryService(mockCategoryRepository)

			err := categoryService.DeleteCategory(context.Background(), testOrganizationId, category.Id)
			assert.Nil(t, err)
		})

		t.Run("returns error when category deletion fails", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepositoryInterface(t)
			mockCategoryRepository.EXPECT().DeleteCategory(context.Background(), testOrganizationId, category.Id).Return(assert.AnError)
			categoryService := NewCategoryService(mockCategoryRepository)

			err := categoryService.DeleteCategory(context.Background(), testOrganizationId, category.Id)
			assert.Error(t, err)
		})
	})

	t.Run("GetCategories", func(t *testing.T) {

		t.Run("returns categories when categories are fetched successfully", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepositoryInterface(t)
			mockCategoryRepository.EXPECT().GetCategories(context.Background(), testOrganizationId).Return([]models.Category{category}, nil)
			categoryService := NewCategoryService(mockCategoryRepository)

			categories, err := categoryService.GetCategories(context.Background(), testOrganizationId)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(categories))
		})

		t.Run("returns error when categories fetch fails", func(t *testing.T) {
			mockCategoryRepository := mocks.NewMockCategoryRepositoryInterface(t)
			mockCategoryRepository.EXPECT().GetCategories(context.Background(), testOrganizationId).Return(nil, assert.AnError)
			categoryService := NewCategoryService(mockCategoryRepository)

			categories, err := categoryService.GetCategories(context.Background(), testOrganizationId)
			assert.Error(t, err)
			assert.Nil(t, categories)
		})
	})
}
