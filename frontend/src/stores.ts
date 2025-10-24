import {defineStore} from "pinia";
import {ref} from "vue";
import {config, updater} from "../wailsjs/go/models";
import {GetConfig, GetServerVersion} from "../wailsjs/go/limbonia/LimboniaApp";

export const useLauncherVersion = defineStore("launcher", () => {
    const serverState = ref<updater.UpdateResponse>()
    const configState = ref<config.Config>()

    const update = async () => {
        try {
            serverState.value = await GetServerVersion()
            configState.value = await GetConfig()
        }catch (error) {

        }
    }

    return {serverState, configState, update}

})