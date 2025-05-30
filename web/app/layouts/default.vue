<template>
  <div>
    <nav>
      <ul>
        <li><NuxtLink to="/">Home</NuxtLink></li>
        <li><NuxtLink to="/protected">Protected</NuxtLink></li>
      </ul>
    </nav>
    <slot />
  </div>
</template>
