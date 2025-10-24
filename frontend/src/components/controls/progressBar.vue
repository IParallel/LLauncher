<script setup lang="ts">
const props = defineProps<{
  value: number   // current progress (0–100)
}>()
</script>

<template>
  <div draggable="false" class="relative w-full bg-[rgb(27,17,3)] rounded h-6 overflow-hidden border-[2px] border-[rgb(66,36,4)] font-[MikoDacs]">
    <div
        class="bg-amber-400 h-6 transition-all duration-300"
        :style="{ width: value + '%' }"
    >
    </div>
    <div class="absolute inset-0 text-[#FFD799] [-webkit-text-stroke:1px_black]">
      {{ value }}%
    </div>
  </div>
</template>

<style scoped>

</style>