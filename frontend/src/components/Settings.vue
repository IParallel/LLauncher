<template>
  <div class="page-root">
    <div class="page-card">

      <!-- Header -->
      <div class="card-header">
        <div class="header-rule-left"></div>
        <span class="header-title">Settings</span>
        <div class="header-rule-right"></div>
      </div>

      <!-- Limbus folder row -->
      <div class="setting-section">
        <div class="section-label">
          <span class="section-icon">◈</span>
          <span>Limbus Company Path</span>
        </div>
        <div
          class="folder-row"
          @click="findLimbusFolder"
          :class="appState.configState?.limbus_folder ? 'row-ok' : 'row-missing'"
        >
          <span class="folder-indicator">
            {{ appState.configState?.limbus_folder ? '✦' : '✕' }}
          </span>
          <span class="folder-path">
            {{ appState.configState?.limbus_folder || "Not set — click to browse" }}
          </span>
          <span class="folder-edit-hint">[ edit ]</span>
        </div>
      </div>

      <!-- Divider -->
      <div class="lc-glow-line"></div>

      <!-- Folder shortcuts -->
      <div class="setting-section">
        <div class="section-label">
          <span class="section-icon">◈</span>
          <span>Folder Shortcuts</span>
        </div>
        <div class="shortcut-grid">
          <Lbutton button-text="App Folder"           :event-func="OpenSettingsFolder" />
          <Lbutton button-text="Limbus Company Folder" :event-func="OpenLimbusFolder" />
          <Lbutton button-text="Limbonia Config Folder" :event-func="OpenLimboniaFolder" />
        </div>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { useToast } from "vue-toastification";
import { OpenFileDialog } from "../../wailsjs/go/limbonia/LimboniaApp";
import { useLauncherVersion } from "../stores";
import Lbutton from "./controls/lbutton.vue";
import { OpenLimboniaFolder, OpenLimbusFolder, OpenSettingsFolder } from "../../wailsjs/go/main/App";

const appState = useLauncherVersion()
const toast = useToast()

const findLimbusFolder = async () => {
  try {
    const result = await OpenFileDialog();
    if (appState.configState) {
      appState.configState.limbus_folder = result
      toast.success("Limbus company folder set")
    }
  } catch (e: any) {
    toast.error(e)
  }
}
</script>

<style scoped>
.page-root {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  width: 100%;
  padding: 1rem;
}

.page-card {
  display: flex;
  flex-direction: column;
  gap: 1.4rem;
  width: min(520px, 100%);
  padding: 1.8rem 2rem;
  background: linear-gradient(160deg, rgba(18,6,6,0.97) 0%, rgba(10,3,3,0.97) 100%);
  border: 1px solid var(--lc-border);
  border-radius: 2px;
  box-shadow: 0 8px 40px rgba(0,0,0,0.8), inset 0 0 30px rgba(0,0,0,0.3);
}

/* Header */
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

/* Section */
.setting-section {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}
.section-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-family: "MikoDacs", sans-serif;
  font-size: 0.72rem;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: var(--lc-text-muted);
}
.section-icon {
  color: var(--lc-gold-dim);
  font-size: 0.65rem;
}

/* Folder row */
.folder-row {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  padding: 0.6rem 0.9rem;
  border: 1px solid var(--lc-border);
  border-radius: 2px;
  background: rgba(0,0,0,0.3);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.folder-row:hover {
  border-color: var(--lc-border-hover);
  background: rgba(120,20,20,0.1);
}
.folder-row.row-ok { border-left: 3px solid var(--lc-green-bright); }
.folder-row.row-missing { border-left: 3px solid var(--lc-red); }

.folder-indicator {
  font-size: 0.9rem;
  flex-shrink: 0;
}
.row-ok    .folder-indicator { color: var(--lc-green-bright); }
.row-missing .folder-indicator { color: var(--lc-red-bright); }

.folder-path {
  flex: 1;
  font-family: "Noto Sans", monospace;
  font-size: 0.78rem;
  color: var(--lc-text);
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.folder-edit-hint {
  font-family: "MikoDacs", sans-serif;
  font-size: 0.65rem;
  letter-spacing: 0.1em;
  color: var(--lc-text-muted);
  flex-shrink: 0;
  transition: color 0.15s;
}
.folder-row:hover .folder-edit-hint { color: var(--lc-gold); }

/* Shortcuts */
.shortcut-grid {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}
</style>
