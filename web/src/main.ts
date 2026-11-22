import '@fontsource/hanken-grotesk/400.css'
import '@fontsource/hanken-grotesk/500.css'
import '@fontsource/hanken-grotesk/600.css'
import '@fontsource/newsreader/400.css'
import '@fontsource/newsreader/400-italic.css'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import 'vuetify/styles'
import './styles/main.scss'
import App from './app/App.vue'
import { restoreSession } from './lib/auth'
import { router } from './app/router'
import { vuetify } from './lib/theme'

restoreSession().finally(() => {
  createApp(App)
    .use(createPinia())
    .use(VueQueryPlugin, {
      queryClientConfig: {
        defaultOptions: { queries: { staleTime: 15_000, retry: (n, e) => n < 1 && !(e as { status?: number }).status } },
      },
    })
    .use(vuetify)
    .use(router)
    .mount('#app')
})
