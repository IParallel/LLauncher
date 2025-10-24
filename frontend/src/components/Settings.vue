<template>
  <div class="flex flex-col gap-20 p-5 font-[Noto_Sans_,_sans-serif]">
    <div class="flex gap-4">
      <p>Limbus Company folder: <span @click="findLimbusFolder" class="text-amber-300 m-1 hover:cursor-pointer">{{ appState.configState?.limbus_folder === "" ? "❌ Not set click to change" : "✅ " + appState.configState?.limbus_folder }}</span></p>
    </div>
    <div class="flex flex-col gap-3">
      <Lbutton button-text="Open App Folder" :event-func="OpenSettingsFolder"></Lbutton>
      <lbutton button-text="Open Limbus Company Folder" :event-func="OpenLimbusFolder"></lbutton>
      <lbutton button-text="Open Limbonia Settings Folder" :event-func="OpenLimboniaFolder"></lbutton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useToast } from "vue-toastification";
import { OpenFileDialog } from "../../wailsjs/go/limbonia/LimboniaApp";
import {useLauncherVersion} from "../stores";
import {onBeforeMount, ref} from "vue";
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
  }catch (e: any) {
    toast.error(e)
  }
}


</script>

<style scoped>

</style>