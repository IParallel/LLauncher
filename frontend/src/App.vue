<script lang="ts" setup>
import { ref, onMounted, onUnmounted, computed } from "vue";
import Sidebar from "./components/Sidebar.vue";
import TitleBar from "./components/TitleBar.vue";
import { CheckForUpdate } from "../wailsjs/go/limbonia/LimboniaApp";
import { DownloadLauncher, LauncherUpdateState, RestartLauncher } from "../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../wailsjs/runtime/runtime";

// Launcher self-update, reported entirely through this status bar.
//
// There used to be a blocking "New Update!" dialog the moment a download
// finished, immediately followed by the app exiting — a modal to dismiss and
// then the window disappearing. The same information now lives here, and
// restarting is the user's call.
interface LauncherUpdate {
  state: "" | "downloading" | "ready" | "error";
  version: string;
  percent: number;
  message: string;
}

const updateAvailable = ref(false);
const upd = ref<LauncherUpdate>({ state: "", version: "", percent: 0, message: "" });

const busy = computed(() => upd.value.state === "downloading");

onMounted(async () => {
  // Pull current state first: a download kicked off at startup may already have
  // finished before this component mounted and started listening.
  try {
    upd.value = (await LauncherUpdateState()) as LauncherUpdate;
  } catch (_) {}

  EventsOn("launcher:update", (s: LauncherUpdate) => {
    upd.value = s;
  });

  try {
    updateAvailable.value = await CheckForUpdate();
  } catch (_) {}
});

onUnmounted(() => EventsOff("launcher:update"));

const applyUpdate = async () => {
  if (busy.value) return;
  try {
    await DownloadLauncher();
  } catch (_) {
    // The Go side already emitted a user-facing error state.
  }
};

const restart = async () => {
  try {
    await RestartLauncher();
  } catch (_) {}
};

const label = computed(() => {
  switch (upd.value.state) {
    case "downloading":
      return upd.value.percent > 0
        ? `DOWNLOADING UPDATE ··· ${upd.value.percent}%`
        : "DOWNLOADING UPDATE ···";
    case "ready":
      return `✦ UPDATE ${upd.value.version} INSTALLED — CLICK TO RESTART`;
    case "error":
      return `UPDATE FAILED — CLICK TO RETRY`;
    default:
      return "✦ UPDATE AVAILABLE — CLICK TO INSTALL";
  }
});
</script>

<template>
  <!-- Root wrapper: full-screen, dark parchment texture.
       Column layout so the custom title bar spans the whole window (including
       over the sidebar), the way Mephi's does — the old decorative bar only
       covered the content pane, which reads wrong once the window is frameless. -->
  <main class="lc-bg-texture flex flex-col h-screen w-screen overflow-hidden select-none">

    <TitleBar />

    <div class="flex flex-row flex-1 overflow-hidden">

      <!-- Sidebar -->
      <Sidebar />

      <!-- Main content pane -->
      <div class="relative flex-1 flex flex-col overflow-hidden">

      <!-- Page content -->
      <div class="flex-1 overflow-auto animate-fadein">
        <RouterView />
      </div>

      <!-- Bottom status bar -->
      <div class="flex items-center px-5 py-1 border-t border-[var(--lc-border)] bg-[var(--lc-panel)] gap-3">

        <!-- Update: available, downloading, installed-awaiting-restart, or failed.
             All of it reported here rather than in a modal. -->
        <template v-if="updateAvailable || upd.state">
          <span
            class="w-2 h-2 rounded-full"
            :class="upd.state === 'error'
              ? 'bg-[var(--lc-red-bright,#d94a4a)] shadow-[0_0_6px_#d94a4a]'
              : 'bg-[var(--lc-gold-bright)] animate-breath shadow-[0_0_6px_var(--lc-gold-bright)]'"
          ></span>

          <button
            @click="upd.state === 'ready' ? restart() : applyUpdate()"
            :disabled="busy"
            class="update-btn"
            :class="{ 'update-btn-error': upd.state === 'error' }"
          >
            {{ label }}
          </button>

          <!-- Thin progress line, only while actually downloading. -->
          <span v-if="busy && upd.percent > 0" class="update-track">
            <span class="update-fill" :style="{ width: upd.percent + '%' }"></span>
          </span>

          <span v-if="upd.state === 'error' && upd.message"
                class="text-[10px] text-[var(--lc-text-muted)] font-[MikoDacs] tracking-widest">
            {{ upd.message }}
          </span>
        </template>

        <!-- Normal -->
        <template v-else>
          <span class="w-2 h-2 rounded-full bg-[var(--lc-green-bright)] shadow-[0_0_6px_var(--lc-green-bright)]"></span>
          <span class="text-[10px] text-[var(--lc-text-muted)] font-[MikoDacs] tracking-widest uppercase">System Ready</span>
        </template>

        <div class="flex-1"></div>
        <span class="text-[10px] text-[var(--lc-text-muted)] font-[MikoDacs] tracking-widest">© IBello</span>
      </div>
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
.update-btn-error {
  color: #d94a4a;
  text-shadow: 0 0 8px rgba(217, 74, 74, 0.4);
}
.update-btn-error:hover {
  color: #fff;
  text-shadow: 0 0 12px rgba(217, 74, 74, 0.8);
}
.update-track {
  display: inline-block;
  width: 80px;
  height: 2px;
  background: rgba(255, 255, 255, 0.12);
  border-radius: 2px;
  overflow: hidden;
}
.update-fill {
  display: block;
  height: 100%;
  background: var(--lc-gold-bright);
  transition: width 120ms linear;
}
</style>
