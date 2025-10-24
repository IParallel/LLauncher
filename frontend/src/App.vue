<script lang="ts" setup>
import Sidebar from "./components/Sidebar.vue";
</script>

<template>
  <main class="flex flex-row gap-1 m-1 h-[calc(100vh-0.25rem*2)]">
      <Sidebar/>
    <div class="flex-1 bg-[rgb(14,14,14)] rounded-[0_5px_5px_0]">
      <RouterView />
    </div>
  </main>
</template>

<style>
</style>
