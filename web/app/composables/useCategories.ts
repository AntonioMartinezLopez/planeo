import type { Category } from "~/clients/core/types.gen";

export default function (categories: Category[]) {
  const getCategoryById = (id: number) => {
    return categories.find(category => category.Id === id);
  };

  return {
    getCategoryById,
  };
}
