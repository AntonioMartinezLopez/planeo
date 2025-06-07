<script setup lang="ts">
import { Calendar, GalleryVerticalEnd, Inbox, ListTodo, Settings, Users } from 'lucide-vue-next'
import { useSidebar } from './ui/sidebar'

const { toggleSidebar } = useSidebar()

// Menu items.
const items = [
  {
    title: 'Requests',
    url: '/',
    icon: Inbox,
  },
  {
    title: 'Tasks',
    url: '/tasks',
    icon: ListTodo,
  },
  {
    title: 'Planning',
    url: '/planning',
    icon: Calendar,
  },
  {
    title: 'Group Management',
    url: '/group',
    icon: Users,
  },
  {
    title: 'Settings',
    url: '/settings',
    icon: Settings,
  },
]
</script>

<template>
  <Sidebar
    variant="sidebar"
    collapsible="icon"
  >
    <SidebarHeader>
      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton
            size="lg"
            class="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            @click="toggleSidebar"
          >
            <div class="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
              <component
                :is="GalleryVerticalEnd"
                class="size-4"
              />
            </div>
            <div class="flex-1 text-left text-lg leading-tight">
              <span class="truncate font-medium">
                Planeo
              </span>
            </div>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
    </SidebarHeader>
    <SidebarContent>
      <SidebarGroup>
        <SidebarGroupContent>
          <SidebarMenu>
            <SidebarMenuItem
              v-for="item in items"
              :key="item.title"
            >
              <SidebarMenuButton
                as-child
              >
                <NuxtLink :href="item.url">
                  <component :is="item.icon" />
                  <span>{{ item.title }}</span>
                </NuxtLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>
    </SidebarContent>

    <SidebarFooter>
      <AppNavUser />
    </SidebarFooter>
  </Sidebar>
</template>
