<script setup lang="ts">
import SidebarButton from "./controls/SidebarButton.vue";
</script>

<template>
  <aside class="relative flex flex-col bg-[var(--lc-panel)] border-r border-[var(--lc-border)] w-[150px] shrink-0 overflow-hidden">


    <!-- No logo block: the title bar carries the brand now (as Mephi's does), and
         repeating it here would eat most of the vertical space in a 440px window. -->
    <nav class="flex flex-col gap-1 p-2 flex-1 pt-3">
      <SidebarButton route-link="/"           button-text="Mephi"      icon="⬡"/>
      <SidebarButton route-link="/settings"   button-text="Settings"   icon="✦"/>
    </nav>

    
    <div class="lc-glow-line mx-3 mb-1"></div>
    <div class="py-2 px-3">
      <p class="font-[MikoDacs] text-[8px] text-[var(--lc-text-muted)] tracking-widest uppercase text-center">Est. 2026</p>
    </div>

    
    <div class="absolute right-0 top-0 h-full w-[2px] bg-gradient-to-b from-transparent via-[var(--lc-gold-dim)] to-transparent opacity-40"></div>
  </aside>
</template>

<!-- Styles removed with the logo block they belonged to. -->