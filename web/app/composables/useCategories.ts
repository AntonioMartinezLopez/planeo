import type { Category } from "~/clients/core/types.gen";

export default function (categories: Category[]) {
  const getCategoryById = (id: number) => {
    return categories.find(category => category.Id === id);
  };

  const getCategoryIdByName = (label: string) => {
    const category = categories.find(category => category.Label === label);
    return category?.Id;
  };

  return {
    getCategoryById,
    getCategoryIdByName,
  };
}
