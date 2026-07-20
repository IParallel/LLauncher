import {createApp} from 'vue'
import App from './App.vue'
import './style.css';

import {createRouter, createWebHashHistory} from "vue-router";
import Limbonia from "./components/Limbonia.vue";
import Settings from "./components/Settings.vue";
import {createPinia} from "pinia";
import {useLauncherVersion} from "./stores";
import Toast from "vue-toastification";
import "vue-toastification/dist/index.css";

const routes = [
    {
        path: '/',
        component: Limbonia,
    },
    {
        path: '/settings',
        component: Settings,
    },
]

const router = createRouter({
    history: createWebHashHistory(),
    routes,
})

const pinia = createPinia()

router.beforeEach(async (to, from, next) => {
    const config = useLauncherVersion()
    if (!config.configState || !config.serverState) {
        await config.update()
    }
    next()
})

createApp(App).use(router).use(pinia).use(Toast, {
    timeout: 3000,
    closeOnClick: true,
    pauseOnFocusLoss: true,
    pauseOnHover: true,
    draggable: true,
    draggablePercent: 0.6,
    showCloseButtonOnHover: false,
    hideProgressBar: true,
    closeButton: "button",
    icon: true,
    rtl: false,
}).mount('#app')
