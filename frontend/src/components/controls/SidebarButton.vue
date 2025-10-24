<script setup lang="ts">
const props = defineProps({
  buttonText: {type: String, default: "ButtonText"},
  routeLink: {type: String, default: "Home"}
})

</script>

<template>
  <RouterLink
      draggable="false"
      :to=routeLink
      active-class="active"
      class="btn select-none flex flex-1 justify-center items-center w-full h-[50px]
       hover:cursor-pointer border-[8px] border-[var(--button-border-color)] hover:border-[var(--button-border-color-hover)]
       active:scale-95 transition-all"
  >
    {{buttonText}}
  </RouterLink>
</template>

<style scoped>

.btn {
  border-radius: 0.3rem;
  font-size: 1.5rem;
  font-family: "MikoDacs", sans-serif;
  background-color: rgb(27, 17, 3);
  color: #FFD799;
}
.active {
  box-shadow: inset 0px 0px 4px 1px #000;
  border-color: #884207;
  color: #FCC600;
  background-color: #580100;
}
</style>