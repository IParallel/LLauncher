<script setup lang="ts">

const props = defineProps({
  buttonText: {type: String, default: "ButtonText"},
  eventFunc: {type: Function, default: null},
})

</script>

<template>
  <button draggable="false" @click="eventFunc && eventFunc()" 
          class="hover:cursor-pointer border-[5px] border-[rgb(66,36,4)] p-1
           hover:border-[rgb(86,56,12)] text-[#FFD799]
            active:text-amber-400 active:scale-95">{{buttonText}}</button>
</template>

<style scoped>
button {
  border-radius: 0.3rem;
  font-size: 1.5rem;
  font-family: "MikoDacs", sans-serif;
  background-color: rgb(27, 17, 3);
}
</style>