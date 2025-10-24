<script setup lang="ts">

import SidebarButton from "./controls/SidebarButton.vue";
</script>

<template>
  <div class="flex flex-col items-center gap-2 p-2 bg-[rgb(15,15,15)] w-[clamp(150px,20%,200px)] rounded-[5px_0_0_5px]">
    <SidebarButton route-link="/" button-text="Limbonia"/>
    <SidebarButton route-link="/bot-quixote" button-text="BotQuixote"/>
    <SidebarButton route-link="/settings" button-text="Settings"/>
    <SidebarButton route-link="/help" button-text="Help"/>
  </div>
</template>

<style scoped>

</style>