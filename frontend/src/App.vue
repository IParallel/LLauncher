<script lang="ts" setup>
import { ref, onMounted } from "vue";
import Sidebar from "./components/Sidebar.vue";
import { CheckForUpdate } from "../wailsjs/go/limbonia/LimboniaApp";
import { DownloadLauncher } from "../wailsjs/go/main/App";

const updateAvailable = ref(false);
const downloading = ref(false);

onMounted(async () => {
  try {
    updateAvailable.value = await CheckForUpdate();
  } catch (_) {}
});

const applyUpdate = async () => {
  if (downloading.value) return;
  downloading.value = true;
  try {
    await DownloadLauncher();
  } finally {
    downloading.value = false;
  }
};
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

        <!-- Update available -->
        <template v-if="updateAvailable">
          <span class="w-2 h-2 rounded-full bg-[var(--lc-gold-bright)] animate-breath shadow-[0_0_6px_var(--lc-gold-bright)]"></span>
          <button
            @click="applyUpdate"
            :disabled="downloading"
            class="update-btn"
          >
            {{ downloading ? 'DOWNLOADING···' : '✦ UPDATE AVAILABLE — CLICK TO INSTALL' }}
          </button>
        </template>

        <!-- Normal -->
        <template v-else>
          <span class="w-2 h-2 rounded-full bg-[var(--lc-green-bright)] shadow-[0_0_6px_var(--lc-green-bright)]"></span>
          <span class="text-[10px] text-[var(--lc-text-muted)] font-[MikoDacs] tracking-widest uppercase">System Ready</span>
        </template>

        <div class="flex-1"></div>
        <span class="text-[10px] text-[var(--lc-text-muted)] font-[MikoDacs] tracking-widest">© LCorp</span>
      </div>
    </div>
  </main>
</template>

<style>
.update-btn {
  font-family: "MikoDacs", sans-serif;
  font-size: 10px;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: var(--lc-gold-bright);
  background: transparent;
  border: none;
  padding: 0;
  cursor: pointer;
  transition: color 0.15s, text-shadow 0.15s;
  text-shadow: 0 0 8px rgba(245, 204, 48, 0.4);
}
.update-btn:hover {
  color: #fff;
  text-shadow: 0 0 12px rgba(245, 204, 48, 0.8);
}
.update-btn:disabled {
  color: var(--lc-text-muted);
  cursor: default;
  text-shadow: none;
}
</style>
