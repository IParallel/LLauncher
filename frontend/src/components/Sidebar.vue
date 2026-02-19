<script setup lang="ts">
import SidebarButton from "./controls/SidebarButton.vue";
</script>

<template>
  <aside class="relative flex flex-col bg-[var(--lc-panel)] border-r border-[var(--lc-border)] w-[180px] shrink-0 overflow-hidden">

    
    <div class="flex flex-col items-center justify-center py-6 px-3 border-b border-[var(--lc-border)] gap-1">
      
      <div class="relative w-16 h-16 flex items-center justify-center">
        <svg viewBox="0 0 64 64" class="absolute inset-0 w-full h-full" fill="none" xmlns="http://www.w3.org/2000/svg">
          
          <polygon
            points="32,3 57.98,17.5 57.98,46.5 32,61 6.02,46.5 6.02,17.5"
            stroke="var(--lc-border-active)" stroke-width="1.5" fill="var(--lc-panel-light)"
          />
          
          <polygon
            points="32,9 52.85,21 52.85,43 32,55 11.15,43 11.15,21"
            stroke="var(--lc-gold-dim)" stroke-width="0.7" fill="none" opacity="0.5"
          />
          
          <circle cx="32"    cy="9"  r="1" fill="var(--lc-gold-dim)" opacity="0.6"/>
          <circle cx="52.85" cy="21" r="1" fill="var(--lc-gold-dim)" opacity="0.6"/>
          <circle cx="52.85" cy="43" r="1" fill="var(--lc-gold-dim)" opacity="0.6"/>
          <circle cx="32"    cy="55" r="1" fill="var(--lc-gold-dim)" opacity="0.6"/>
          <circle cx="11.15" cy="43" r="1" fill="var(--lc-gold-dim)" opacity="0.6"/>
          <circle cx="11.15" cy="21" r="1" fill="var(--lc-gold-dim)" opacity="0.6"/>
        </svg>
        
        <span class="logo-l pt-[3px] pl-[1px]">L</span>
      </div>
      <span class="font-[MikoDacs] text-[var(--lc-gold-bright)] text-xs tracking-[0.25em] uppercase mt-1">L·Launcher</span>
      <span class="font-[MikoDacs] text-[var(--lc-text-muted)] text-[9px] tracking-widest uppercase">Limbus Corp.</span>
    </div>

    
    <nav class="flex flex-col gap-1 p-2 flex-1">
      <SidebarButton route-link="/"           button-text="Limbonia"   icon="⬡"/>
      <SidebarButton route-link="/bot-quixote" button-text="BotQuixote" icon="⚙"/>
      <SidebarButton route-link="/settings"   button-text="Settings"   icon="✦"/>
      <SidebarButton route-link="/help"       button-text="Help"       icon="?"/>
    </nav>

    
    <div class="lc-glow-line mx-3 mb-1"></div>
    <div class="py-2 px-3">
      <p class="font-[MikoDacs] text-[8px] text-[var(--lc-text-muted)] tracking-widest uppercase text-center">Est. 2026</p>
    </div>

    
    <div class="absolute right-0 top-0 h-full w-[2px] bg-gradient-to-b from-transparent via-[var(--lc-gold-dim)] to-transparent opacity-40"></div>
  </aside>
</template>

<style scoped>
@keyframes logo-glow {
  0%, 100% {
    text-shadow: 0 0 6px rgba(192,25,42,0.4), 0 0 12px rgba(212,160,23,0.2);
  }
  50% {
    text-shadow: 0 0 16px rgba(192,25,42,1), 0 0 32px rgba(212,160,23,0.8), 0 0 48px rgba(192,25,42,0.4);
  }
}
.logo-l {
  position: relative;
  z-index: 10;
  font-family: "MikoDacs", sans-serif;
  font-size: 1.6rem;
  line-height: 1;
  color: var(--lc-gold);
  animation: logo-glow 4s ease-in-out infinite;
}
</style>