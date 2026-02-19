<script setup lang="ts">
const props = defineProps<{
  value: number
}>()
</script>

<template>
  <div class="progress-wrap" draggable="false">
    <!-- Track -->
    <div class="progress-track">
      <!-- Fill -->
      <div class="progress-fill" :style="{ width: value + '%' }">
        <!-- Animated shimmer inside fill -->
        <span class="fill-sheen"></span>
      </div>
      <!-- Segmented tick marks overlay -->
      <div class="ticks">
        <span v-for="n in 9" :key="n" class="tick"
              :style="{ left: (n * 10) + '%' }"></span>
      </div>
    </div>
    <!-- Label row -->
    <div class="progress-label">
      <span>Downloading…</span>
      <span class="progress-pct">{{ value }}%</span>
    </div>
  </div>
</template>

<style scoped>
.progress-wrap {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  width: 100%;
}
.progress-track {
  position: relative;
  width: 100%;
  height: 20px;
  background: var(--lc-panel-light);
  border: 1px solid var(--lc-border);
  border-radius: 2px;
  overflow: hidden;
  box-shadow: inset 0 2px 4px rgba(0,0,0,0.6);
}
.progress-fill {
  position: absolute;
  left: 0; top: 0; bottom: 0;
  background: linear-gradient(90deg, #6b0f18, #c0192a, #d4a017, #c0192a);
  background-size: 200% 100%;
  animation: shimmer-fill 1.8s linear infinite;
  transition: width 0.3s ease;
  border-radius: 1px;
}
@keyframes shimmer-fill {
  from { background-position: 0% 0; }
  to   { background-position: 200% 0; }
}
.fill-sheen {
  position: absolute;
  top: 0; left: 0; right: 0;
  height: 50%;
  background: rgba(255,255,255,0.06);
  border-radius: 1px 1px 0 0;
}
.ticks {
  position: absolute;
  inset: 0;
  pointer-events: none;
}
.tick {
  position: absolute;
  top: 20%;
  bottom: 20%;
  width: 1px;
  background: rgba(0,0,0,0.25);
}
.progress-label {
  display: flex;
  justify-content: space-between;
  font-family: "MikoDacs", sans-serif;
  font-size: 0.72rem;
  letter-spacing: 0.06em;
  color: var(--lc-text-dim);
  text-transform: uppercase;
}
.progress-pct {
  color: var(--lc-gold);
}
</style>