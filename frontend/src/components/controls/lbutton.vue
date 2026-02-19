<script setup lang="ts">
const props = defineProps({
  buttonText: { type: String, default: "ButtonText" },
  eventFunc:  { type: Function, default: null },
})
</script>

<template>
  <button
      draggable="false"
      @click="eventFunc && eventFunc()"
      class="lc-btn group relative overflow-hidden"
  >
    <!-- Background shimmer on hover -->
    <span class="shimmer"></span>

    <!-- Top edge highlight -->
    <span class="absolute top-0 left-0 right-0 h-[1px] bg-gradient-to-r
                 from-transparent via-[var(--lc-gold-dim)] to-transparent opacity-60"></span>

    <!-- Label -->
    <span class="relative z-10">{{ buttonText }}</span>

    <!-- Bottom edge shadow -->
    <span class="absolute bottom-0 left-0 right-0 h-[1px] bg-black opacity-50"></span>
  </button>
</template>

<style scoped>
.lc-btn {
  font-family: "MikoDacs", sans-serif;
  font-size: 1.1rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--lc-cream);
  background: linear-gradient(180deg, #200808 0%, #130404 100%);
  border: 1px solid var(--lc-border);
  border-bottom-color: #0e0202;
  border-radius: 2px;
  padding: 0.5rem 1.4rem;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s, box-shadow 0.15s, transform 0.1s;
  box-shadow: 0 2px 6px rgba(0,0,0,0.7), inset 0 1px 0 rgba(212,160,23,0.07);
  width: 100%;
}
.lc-btn:hover {
  color: var(--lc-gold-bright);
  border-color: var(--lc-border-hover);
  box-shadow: 0 2px 12px rgba(0,0,0,0.8), 0 0 10px rgba(192,25,42,0.35),
              inset 0 1px 0 rgba(212,160,23,0.12);
}
.lc-btn:active {
  transform: scale(0.97);
  color: var(--lc-gold);
  border-color: var(--lc-border-active);
  box-shadow: inset 0 2px 6px rgba(0,0,0,0.8);
}
.shimmer {
  position: absolute;
  inset: 0;
  background: linear-gradient(105deg, transparent 40%, rgba(255,200,80,0.06) 50%, transparent 60%);
  opacity: 0;
  transition: opacity 0.3s;
}
.lc-btn:hover .shimmer {
  opacity: 1;
}
</style>