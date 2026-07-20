<script setup lang="ts">
import { ref, onMounted } from "vue";
import { WindowMinimise, Quit } from "../../wailsjs/runtime/runtime";
import { LauncherVersion } from "../../wailsjs/go/main/App";
import logo from "../assets/images/logo_rounded_512.png";

// Custom title bar for the frameless window, mirroring Mephi's approach:
// the whole bar is a drag region (--wails-draggable), and anything clickable
// opts back out with no-drag so it stays a button rather than a grab handle.
//
// With Frameless there is no OS chrome, so these controls are the ONLY way to
// move, minimise or close the window.
const version = ref("");

onMounted(async () => {
  try {
    version.value = await LauncherVersion();
  } catch (_) {
    // Cosmetic only — the bar still works without it.
  }
});
</script>

<template>
  <!-- No space after the colon, deliberately: the v2 runtime compares the computed
       custom-property value against "drag" with a strict !== and no .trim(), so a
       retained leading space would silently make the window immovable. -->
  <div class="titlebar" style="--wails-draggable:drag">
    <!-- draggable=false matters here: an <img> is natively draggable, and inside a
         --wails-draggable region that would start an HTML image drag instead of
         moving the window. -->
    <img :src="logo" alt="" class="logo" draggable="false" />
    <span class="brand">L·Launcher</span>
    <span v-if="version" class="version">{{ version }}</span>

    <!-- Unconditional spacer, pushing the window controls to the right edge.
         Must NOT be breakpoint-gated: the window is 600px wide, below Tailwind's
         sm: (640px), so a `hidden sm:block` spacer collapses to display:none and
         everything packs against the left edge. -->
    <div class="spacer"></div>

    <div class="winbtns" style="--wails-draggable:no-drag">
      <button class="wbtn" title="Minimise" @click="WindowMinimise()">─</button>
      <button class="wbtn close" title="Close" @click="Quit()">✕</button>
    </div>
  </div>
</template>

<style scoped>
.titlebar {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  padding-left: 12px;
  background: var(--lc-panel);
  border-bottom: 1px solid var(--lc-border);
  flex: none;
  user-select: none;
}

.logo {
  width: 20px;
  height: 20px;
  flex: none;
  object-fit: contain;
}

.brand {
  font-family: "MikoDacs", sans-serif;
  font-size: 11px;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  color: var(--lc-gold-bright);
  text-shadow: 0 0 8px rgba(245, 204, 48, 0.25);
}

.version {
  font-family: "MikoDacs", sans-serif;
  font-size: 9px;
  letter-spacing: 0.1em;
  color: var(--lc-text-muted);
}

/* Pushes the window controls to the right edge. min-width keeps a gap even if
   the title text ever grows enough to squeeze it. */
.spacer {
  flex: 1 1 auto;
  min-width: 16px;
}

.winbtns {
  display: flex;
  height: 100%;
}

.wbtn {
  width: 38px;
  height: 100%;
  background: transparent;
  border: none;
  color: var(--lc-text-muted);
  font-size: 11px;
  line-height: 1;
  cursor: pointer;
  transition: background 0.12s, color 0.12s;
}

.wbtn:hover {
  background: rgba(245, 204, 48, 0.1);
  color: var(--lc-gold-bright);
}

.wbtn.close:hover {
  background: #c0192a;
  color: #fff;
}
</style>
