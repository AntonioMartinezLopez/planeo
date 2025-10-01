<script lang="ts" setup>
import type {
  ExpandedState,
  TableOptions,
} from "@tanstack/vue-table";
import type { Category, Request } from "~/clients/core/types.gen";
import {
  FlexRender,
  getCoreRowModel,
  useVueTable,
} from "@tanstack/vue-table";
import { ref } from "vue";
import { Select, SelectContent, SelectItem, SelectTrigger } from "../ui/select";
import { valueUpdater } from "../ui/table/utils";
import { getRequestColumnDefinition } from "./columns";

const props = defineProps<{
  requests: Request[];
  categories: Category[];
  isFirstPage: boolean;
  isLastPage: boolean;
}>();

const emit = defineEmits<{
  (e: "update:selectedCategories", value: number[]): void;
  (e: "nextPage"): void;
  (e: "prevPage"): void;
}>();

const pageSize = defineModel("pageSize", { type: Number });
const selectedCategories = defineModel("selectedCategories", { type: Array<number>, default: () => [] });

const { getCategoryIdByName } = useCategories(props.categories);
const rowSelection = ref({});
const expanded = ref<ExpandedState>({});
const columns = computed(() => getRequestColumnDefinition(props.categories));

const tableOptions = reactive<TableOptions<Request>>({
  get data() {
    return props.requests;
  },
  getCoreRowModel: getCoreRowModel(),
  get columns() {
    return columns.value;
  },
  onRowSelectionChange: updaterOrValue => valueUpdater(updaterOrValue, rowSelection),
  onExpandedChange: updaterOrValue => valueUpdater(updaterOrValue, expanded),
  state: {
    get rowSelection() { return rowSelection.value; },
    get expanded() { return expanded.value; },
  },
});

const table = useVueTable(tableOptions);

function setSelectedCategories(value: Record<number, string>) {
  const categoryIds = Object.values(value).reduce((acc, categoryName) => {
    const categoryId = getCategoryIdByName(categoryName);
    if (categoryId !== undefined) {
      acc.push(categoryId);
    }
    return acc;
  }, [] as number[]);

  emit("update:selectedCategories", categoryIds);
}

// watch(selectedCategories, () => {
//   const mappedCategories = selectedCategories.value.map(cat => getCategoryIdByName(cat));
//   table.setColumnFilters([{ id: "CategoryId", value: mappedCategories }]);
// });
</script>

<template>
  <div class="w-full h-full grid grid-rows-[auto_1fr_auto] items-stretch">
    <div class="flex gap-2 items-center py-4">
      <Select
        multiple

        @update:model-value="(value) => setSelectedCategories"
      >
        <SelectTrigger class="w-[200px]">
          <span v-if="selectedCategories.length">Select categories</span>
          <span v-else>No category selected</span>
          <SelectContent>
            <SelectItem
              v-for="category in props.categories"
              :key="category.Id"
              :value="category.Label"
            >
              {{ category.Label }}
            </SelectItem>
          </SelectContent>
        </SelectTrigger>
      </Select>
    </div>

    <ScrollArea class="rounded-md border h-0 min-h-full">
      <Table>
        <TableHeader>
          <TableRow
            v-for="headerGroup in table.getHeaderGroups()"
            :key="headerGroup.id"
          >
            <TableHead
              v-for="header in headerGroup.headers"
              :key="header.id"
            >
              <FlexRender
                v-if="!header.isPlaceholder"
                :render="header.column.columnDef.header"
                :props="header.getContext()"
              />
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <template v-if="table.getRowModel().rows?.length">
            <template
              v-for="row in table.getRowModel().rows"
              :key="row.id"
            >
              <TableRow
                :data-state="row.getIsSelected() && 'selected'"
              >
                <TableCell
                  v-for="cell in row.getVisibleCells()"
                  :key="cell.id"
                >
                  <FlexRender
                    :render="cell.column.columnDef.cell"
                    :props="cell.getContext()"
                  />
                </TableCell>
              </TableRow>
            </template>
          </template>
          <TableRow v-else>
            <TableCell
              :colspan="columns.length"
              class="h-24 text-center"
            >
              No results.
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </ScrollArea>

    <div class="flex items-center justify-end space-x-2 py-4">
      <div class="flex-1 text-sm text-muted-foreground">
        {{ table.getFilteredSelectedRowModel().rows.length }} of
        {{ table.getFilteredRowModel().rows.length }} row(s) selected.
      </div>
      <div class="flex items-center space-x-2">
        <p class="text-sm font-medium">
          Rows per page
        </p>
        <Select
          v-model="pageSize"
          class="w-[70px]"
        >
          <SelectTrigger class="h-8 w-[70px]">
            <SelectValue :placeholder="`${pageSize}`" />
          </SelectTrigger>
          <SelectContent side="top">
            <SelectItem
              v-for="size in [10, 20, 30, 40, 50]"
              :key="size"
              :value="size"
            >
              {{ size }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="space-x-2">
        <Button
          variant="outline"
          size="sm"
          :disabled="props.isFirstPage"
          @click="emit('prevPage')"
        >
          Previous
        </Button>

        <Button
          variant="outline"
          size="sm"
          :disabled="props.isLastPage"
          @click="emit('nextPage')"
        >
          Next
        </Button>
      </div>
    </div>
  </div>
</template>
