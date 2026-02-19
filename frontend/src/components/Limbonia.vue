<script setup lang="ts">
import {InjectLimbonia} from "../../wailsjs/go/main/App";
import {EventsOn, LogError} from "../../wailsjs/runtime";
import Lbutton from "./controls/lbutton.vue";
import ProgressBar from "./controls/progressBar.vue";
import {computed, onBeforeMount, onUnmounted, ref} from "vue";
import {DownloadLimbonia} from "../../wailsjs/go/limbonia/LimboniaApp";
import {useLauncherVersion} from "../stores";
import { useToast } from "vue-toastification";

const injectLimbo = async () => {
  try {
    if (appState.configState?.limbus_folder === "") {
      toast.error("Please select Limbus Company executable in settings tab")
      return;
    }
    await InjectLimbonia();
    toast.success("Limbonia injecting...")
  } catch (e: any) {
    toast.error(e);
  }
}


// Track progress for each file by path
const injectorProgress = ref(0)
const limboniaDllProgress = ref(0)
const injectorDone = ref(false)
const limboniaDllDone = ref(false)
const downloadingFile = ref("")
const progress = computed(() => Math.round((injectorProgress.value + limboniaDllProgress.value) / 2))
const showProgressBar = ref(false)
const downloading = ref(false)
const hasUpdate = ref(false)
const appState = useLauncherVersion()
const toast = useToast()

const resetDownload = async () => {
  // Brief delay so user sees 100% before bar disappears
  await new Promise(r => setTimeout(r, 600))
  showProgressBar.value = false
  injectorProgress.value = 0
  limboniaDllProgress.value = 0
  injectorDone.value = false
  limboniaDllDone.value = false
  downloadingFile.value = ""
  downloading.value = false
  await appState.update()
  if (appState.serverState?.limbo_version == appState.configState?.current_limbonia_version) {
    hasUpdate.value = false
  }
}

const updateFunc = async () => {
  if (downloading.value) return;
  downloading.value = true;
  showProgressBar.value = true
  injectorProgress.value = 0
  limboniaDllProgress.value = 0
  injectorDone.value = false
  limboniaDllDone.value = false
  downloadingFile.value = ""
  try {
    await DownloadLimbonia()
    await resetDownload()
  } catch (e: any) {
    LogError(e);
    await resetDownload()
  }
}

onBeforeMount(async () => {
  try {
    if (appState.serverState?.limbo_version != appState.configState?.current_limbonia_version) {
      hasUpdate.value = true
    }
  } catch (err) {}
})

const checkInterval = setInterval(async () => {
  if (downloading.value) return;
  await appState.update()
  hasUpdate.value = appState.serverState?.limbo_version != appState.configState?.current_limbonia_version
}, 10_000)

onUnmounted(() => clearInterval(checkInterval))

// Listen for download progress events
EventsOn("download:progress", (payload: any) => {
  if (payload && typeof payload === "object" && typeof payload.file === "string") {
    limboniaDllProgress.value = payload.percent;
    injectorProgress.value = payload.percent;
    downloadingFile.value = payload.file.split(/[\\/]/).pop();
  } else if (typeof payload === "number") {
    injectorProgress.value = payload;
    limboniaDllProgress.value = payload;
    downloadingFile.value = "";
  }
})
</script>

<template>
  <div class="page-root">

    
    <div class="bg-art">
      <img
        draggable="false"
        src="../assets/images/limbonia.png"
        alt=""
        class="bg-art-img"
      />
    </div>

    
    <div class="page-card">

      
      <div class="card-header">
        <div class="header-rule-left"></div>
        <span class="header-title">Limbonia</span>
        <div class="header-rule-right"></div>
      </div>

      
      <p class="page-desc">Cheat menu to customize the limbussy experience</p>

      
      <div class="version-grid">
        <div class="version-row">
          <span class="version-label">Installed Version</span>
          <span
            class="version-value"
            :class="appState.configState?.current_limbonia_version ? 'is-ok' : 'is-err'"
          >
            {{ appState.configState?.current_limbonia_version === "" ? "None" : (appState.configState?.current_limbonia_version ?? "—") }}
          </span>
        </div>

        <div class="lc-rule my-1"></div>

        <div class="version-row">
          <span class="version-label">Latest Version</span>
          <span
            class="version-value"
            :class="appState.serverState ? 'is-ok' : 'is-err'"
          >
            {{ appState.serverState?.limbo_version ?? "Error" }}
          </span>
        </div>

        
        <Transition name="badge-pop">
          <div v-if="hasUpdate" class="update-badge animate-breath">
            <span class="badge-dot"></span>
            <span>New update available</span>
          </div>
        </Transition>
      </div>

      
      <div class="progress-area" v-if="showProgressBar">
        <div v-if="downloadingFile" class="progress-filename">{{ downloadingFile }}</div>
        <ProgressBar :value="progress" />
      </div>

      
      <div class="action-row">
        <div class="action-col" v-if="appState.configState?.current_limbonia_version">
          <lbutton :event-func="injectLimbo" button-text="Open" />
        </div>
        <div class="action-col">
          <lbutton :event-func="updateFunc" button-text="Update" />
        </div>
      </div>

    </div>
  </div>
</template>

<style scoped>
/* Page layout */
.page-root {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  width: 100%;
  overflow: hidden;
}

/* Background art */
.bg-art {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 0;
  overflow: hidden;
}
.bg-art-img {
  position: absolute;
  right: -24px;
  bottom: -16px;
  width: 380px;
  height: 380px;
  opacity: 0.10;
  filter: sepia(0.6) hue-rotate(-10deg);
  user-select: none;
}

/* Main card */
.page-card {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 1.2rem;
  width: min(480px, 90%);
  padding: 1.8rem 2rem;
  background: linear-gradient(160deg, rgba(18,6,6,0.97) 0%, rgba(10,3,3,0.97) 100%);
  border: 1px solid var(--lc-border);
  border-radius: 2px;
  box-shadow: 0 8px 40px rgba(0,0,0,0.8), inset 0 0 30px rgba(0,0,0,0.3),
              0 0 0 1px rgba(255,200,80,0.04);
}

/* Card header */
.card-header {
  display: flex;
  align-items: center;
  gap: 0.8rem;
}
.header-rule-left,
.header-rule-right {
  flex: 1;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--lc-border));
}
.header-rule-right {
  background: linear-gradient(90deg, var(--lc-border), transparent);
}
.header-title {
  font-family: "MikoDacs", sans-serif;
  font-size: 1.5rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--lc-gold);
  text-shadow: 0 0 14px rgba(200,150,44,0.5);
  white-space: nowrap;
}

/* Version grid */
.version-grid {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  padding: 0.8rem 1rem;
  background: rgba(0,0,0,0.25);
  border: 1px solid var(--lc-border);
  border-radius: 2px;
}
.version-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.version-label {
  font-family: "MikoDacs", sans-serif;
  font-size: 0.78rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--lc-text-muted);
}
.version-value {
  font-family: "MikoDacs", sans-serif;
  font-size: 0.92rem;
  letter-spacing: 0.06em;
}
.is-ok  { color: var(--lc-green-bright); text-shadow: 0 0 8px var(--lc-green-bright); }
.is-err { color: var(--lc-red-bright);   text-shadow: 0 0 8px var(--lc-red-bright);   }

/* Update badge */
.update-badge {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin-top: 0.4rem;
  padding: 0.3rem 0.7rem;
  background: rgba(139,26,26,0.2);
  border: 1px solid var(--lc-red);
  border-radius: 2px;
  font-family: "MikoDacs", sans-serif;
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  color: var(--lc-red-bright);
  text-transform: uppercase;
  width: fit-content;
  align-self: center;
}
.badge-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: var(--lc-red-bright);
  box-shadow: 0 0 6px var(--lc-red-bright);
  flex-shrink: 0;
}

/* Badge transition */
.badge-pop-enter-active { transition: all 0.3s; }
.badge-pop-leave-active { transition: all 0.2s; }
.badge-pop-enter-from  { opacity: 0; transform: translateY(-4px) scale(0.9); }
.badge-pop-leave-to    { opacity: 0; transform: scale(0.9); }

/* Progress */
.progress-area {
  margin-top: -0.3rem;
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}
.progress-filename {
  font-family: "MikoDacs", sans-serif;
  font-size: 0.72rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--lc-gold);
  text-align: left;
}

.page-desc {
  font-family: "Noto Sans", sans-serif;
  font-size: 0.78rem;
  color: var(--lc-text-muted);
  letter-spacing: 0.04em;
  text-align: center;
  margin-top: -0.5rem;
}

/* Action row */
.action-row {
  display: flex;
  gap: 0.75rem;
}
.action-col {
  flex: 1;
}
</style>
