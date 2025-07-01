import type { ColumnDef } from "@tanstack/vue-table";
import type { Category, Request } from "~/clients/core/types.gen";
import { ArrowUpDown } from "lucide-vue-next";
import { Badge } from "../ui/badge";
import { Button } from "../ui/button";
import { Checkbox } from "../ui/checkbox";
import DataTableDropdown from "./DataTableDropdown.vue";

export function getRequestColumnDefinition(categories: Category[]) {
  const { getCategoryById } = useCategories(categories);

  const columns: ColumnDef<Request>[] = [
    {
      id: "select",
      header: ({ table }) => h(Checkbox, {
        "modelValue": table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && "indeterminate"),
        "onUpdate:modelValue": value => table.toggleAllPageRowsSelected(!!value),
        "ariaLabel": "Select all",
      }),
      cell: ({ row }) => h(Checkbox, {
        "modelValue": row.getIsSelected(),
        "onUpdate:modelValue": value => row.toggleSelected(!!value),
        "ariaLabel": "Select row",
      }),
      enableSorting: false,
      enableHiding: false,
    },
    {
      accessorKey: "CategoryId",
      header: "Category",
      cell: ({ row }) => {
        const categoryId = row.getValue<number>("CategoryId");
        const category = getCategoryById(categoryId);

        return h(
          Badge,
          {
            variant: "secondary",
            style: `
              border-color: ${category?.Color};
              color: color-mix(in srgb, ${category?.Color} 5%, white 99%);
            `,
          },
          () => (category ? category.Label : "Unknown"),
        );
      },
    },
    {
      accessorKey: "CreatedAt",
      header: "Date",
      cell: ({ row }) => {
        const date = row.getValue("CreatedAt" satisfies keyof Request);

        // console.log("row value", row);
        let formattedDate = "";
        let formattedTime = "";
        if (typeof date === "string") {
          const d = new Date(date);
          formattedDate = d.toLocaleDateString("de-DE");
          formattedTime = d.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" });
        }

        return h("div", { class: "capitalize flex flex-col" }, `${formattedDate} ${formattedTime}`);
      },
    },
    {
      accessorKey: "Email",
      header: ({ column }) => {
        return h(Button, {
          variant: "ghost",
          onClick: () => column.toggleSorting(column.getIsSorted() === "asc"),
        }, () => ["Email", h(ArrowUpDown, { class: "ml-2 h-4 w-4" })]);
      },
      cell: ({ row }) => h("div", { class: "lowercase" }, row.getValue("Email" satisfies keyof Request)),
    },
    {
      accessorKey: "Subject",
      header: () => h("div", { class: "" }, "Subject"),
      cell: ({ row }) => {
      // const amount = Number.parseFloat(row.getValue("Subject" satisfies keyof Request));

        // // Format the amount as a dollar amount
        // const formatted = new Intl.NumberFormat("en-US", {
        //   style: "currency",
        //   currency: "USD",
        // }).format(amount);

        return h("div", { class: "font-medium" }, row.getValue("Subject" satisfies keyof Request));
      },
    },
    {
      id: "actions",
      enableHiding: false,
      header: () => h("div", { class: "" }, "Actions"),
      cell: ({ row }) => {
        const request = row.original;

        return h("div", { class: "relative" }, h(DataTableDropdown, {
          request,
          onExpand: row.toggleExpanded,
        }));
      },
    },
  ];

  return columns;
}
