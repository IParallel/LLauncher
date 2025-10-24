<script setup lang="ts">
import {InjectLimbonia} from "../../wailsjs/go/main/App";
import {EventsOn, LogError} from "../../wailsjs/runtime";
import Lbutton from "./controls/lbutton.vue";
import ProgressBar from "./controls/progressBar.vue";
import {onBeforeMount, ref} from "vue";
import {
  DownloadBotQuixote,
  DownloadLimbonia,
  OpenBotQuixote} from "../../wailsjs/go/limbonia/LimboniaApp";
import {useLauncherVersion} from "../stores";


const appState = useLauncherVersion()

const progress = ref(0)
const showProgressBar = ref(false)
const downloading = ref(false)
const hasUpdate = ref(false)


onBeforeMount(async ()=> {
  try {
    if (appState.serverState?.bot_version != appState.configState?.current_bot_version) {
      hasUpdate.value = true
    }
  } catch (err) {

  }
})

const updateFunc = async () => {
  if (downloading.value) {
    return;
  }
  downloading.value = true;
  try {
    showProgressBar.value = true
    await DownloadBotQuixote()
  }catch (e: any) {
    LogError(e);
  }finally {
    showProgressBar.value = false
    progress.value = 0
    downloading.value = false
    await appState.update()

    if (appState.serverState?.bot_version == appState.configState?.current_bot_version) {
      hasUpdate.value = false
    }
  }
}

EventsOn("download:progress", (percent: number) => {
  progress.value = percent;
})
</script>

<template>
  <div class="relative h-full items-center justify-center content-center z-0">
    <div class="flex flex-col justify-between p-5 h-full">
      <div class="font-[MikoDacs] justify-between">
        <h1>Current Version: <span :class="appState.configState ? 'text-[#00c700]' : 'text-[#FF0000]'">{{appState.configState?.current_bot_version === "" ? "None" : appState.configState?.current_bot_version}}</span></h1>
        <h1>Server Version: <span :class="appState.serverState ? 'text-[#00c700]' : 'text-[#FF0000]'">{{appState.serverState?.bot_version ?? "Error"}}</span></h1>
      </div>
      <progressBar v-if="showProgressBar" :value="progress" class=""/>
      <div class="flex flex-row w-full items-center justify-center gap-10">
        <lbutton v-if="appState.configState?.current_bot_version" class="flex-1/2" :event-func="OpenBotQuixote" button-text="Open" />
        <div class="relative flex-1/2">
          <span v-if="hasUpdate" class="absolute text-[#FF0000] -rotate-45 top-1 -left-5 animate-breath font-[MikoDacs] z-50">New update!</span>
          <lbutton class="w-full" :event-func="updateFunc" button-text="Update" />
        </div>
      </div>
    </div>
    <div class="absolute bottom-0 right-0 -z-1 opacity-25">
      <img width="384" height="384" draggable="false" src="../assets/images/bot_quixote.png">
    </div>
  </div>
</template>

<style scoped>

</style>