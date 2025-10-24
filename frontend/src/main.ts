import {createApp} from 'vue'
import App from './App.vue'
import './style.css';

import {createRouter, createWebHashHistory} from "vue-router";
import Limbonia from "./components/Limbonia.vue";
import BotQuixote from "./components/BotQuixote.vue";
import Settings from "./components/Settings.vue";
import {createPinia} from "pinia";
import {useLauncherVersion} from "./stores";
import Toast from "vue-toastification";
import "vue-toastification/dist/index.css";
import Help from "./components/Help.vue";

const routes = [
    {
        path: '/',
        component: Limbonia,
    },
    {
        path: "/bot-quixote",
        component: BotQuixote,
    },
    {
        path: '/settings',
        component: Settings,
    },
    {
        path: '/help',
        component: Help,
    }
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
