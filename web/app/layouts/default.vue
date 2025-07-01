<template>
  <SidebarProvider>
    <AppSidebar />
    <main class="w-screen h-screen overflow-hidden flex flex-col p-10 scroll-m-10 overflow-y-auto">
      <slot />
    </main>
  </SidebarProvider>
</template>
