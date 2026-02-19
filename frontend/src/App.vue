<script lang="ts" setup>
import Sidebar from "./components/Sidebar.vue";
</script>

<template>
  <!-- Root wrapper: full-screen, dark parchment texture -->
  <main class="lc-bg-texture flex flex-row h-screen w-screen overflow-hidden select-none">

    <!-- Sidebar -->
    <Sidebar />

    <!-- Main content pane -->
    <div class="relative flex-1 flex flex-col overflow-hidden">

      <!-- Top decorative bar -->
      <div class="flex items-center justify-between px-5 py-2 border-b border-[var(--lc-border)] bg-[var(--lc-panel)]">
        <div class="flex items-center gap-2">
          <span class="font-[MikoDacs] text-[var(--lc-gold)] tracking-widest text-sm uppercase opacity-70">DEAR·MANAGER</span>
        </div>
        <div class="lc-glow-line w-24 hidden sm:block"></div>
        <span class="text-[var(--lc-text-muted)] text-xs font-[MikoDacs] tracking-widest uppercase">Limbus Company</span>
      </div>

      <!-- Page content -->
      <div class="flex-1 overflow-auto animate-fadein">
        <RouterView />
      </div>

      <!-- Bottom status bar -->
      <div class="flex items-center px-5 py-1 border-t border-[var(--lc-border)] bg-[var(--lc-panel)] gap-3">
        <span class="w-2 h-2 rounded-full bg-[var(--lc-green-bright)] shadow-[0_0_6px_var(--lc-green-bright)]"></span>
        <span class="text-[10px] text-[var(--lc-text-muted)] font-[MikoDacs] tracking-widest uppercase">System Ready</span>
        <div class="flex-1"></div>
        <span class="text-[10px] text-[var(--lc-text-muted)] font-[MikoDacs] tracking-widest">© LCorp</span>
      </div>
    </div>
  </main>
</template>

<style>
</style>
