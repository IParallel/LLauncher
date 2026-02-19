<script setup lang="ts">
const props = defineProps({
  buttonText: { type: String, default: "ButtonText" },
  routeLink:  { type: String, default: "/" },
  icon:       { type: String, default: "" },
})
</script>

<template>
  <RouterLink
      draggable="false"
      :to="routeLink"
      active-class="is-active"
      class="nav-btn group relative flex items-center gap-3 w-full px-3 py-2.5
             select-none transition-all duration-200 overflow-hidden"
  >
    <!-- Left accent bar (shown on hover/active) -->
    <span class="accent-bar absolute left-0 top-0 h-full w-[3px]
                 bg-gradient-to-b from-transparent via-[var(--lc-gold)] to-transparent
                 opacity-0 group-hover:opacity-100 transition-opacity"></span>

    <!-- Icon -->
    <span v-if="icon" class="icon-slot w-6 h-6 flex items-center justify-center
                              text-[var(--lc-gold-dim)] group-hover:text-[var(--lc-gold)]
                              transition-colors text-sm shrink-0">{{ icon }}</span>

    <!-- Label -->
    <span class="label flex-1 text-left">{{ buttonText }}</span>

    <!-- Hover sheen -->
    <span class="absolute inset-0 bg-[var(--lc-gold)] opacity-0 group-hover:opacity-[0.04] transition-opacity pointer-events-none"></span>
  </RouterLink>
</template>

<style scoped>
.nav-btn {
  font-family: "MikoDacs", sans-serif;
  font-size: 0.95rem;
  letter-spacing: 0.04em;
  color: var(--lc-text-dim);
  border: 1px solid transparent;
  border-radius: 2px;
  background: transparent;
}
.nav-btn:hover {
  color: var(--lc-cream);
  border-color: var(--lc-border);
  background: var(--lc-panel-light);
}
.nav-btn.is-active {
  color: var(--lc-gold-bright);
  background: rgba(140, 20, 20, 0.22);
  border-color: var(--lc-border-active);
  box-shadow: inset 0 0 10px rgba(192,25,42,0.12), var(--lc-glow-red);
}
.nav-btn.is-active .accent-bar {
  opacity: 1;
}
.nav-btn.is-active .icon-slot {
  color: var(--lc-gold-bright);
}
</style>