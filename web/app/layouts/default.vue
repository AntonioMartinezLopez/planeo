<template>
  <SidebarProvider>
    <AppSidebar />
    <main class="w-screen h-screen overflow-hidden flex flex-col p-6">
      <slot />
    </main>
  </SidebarProvider>
</template>
