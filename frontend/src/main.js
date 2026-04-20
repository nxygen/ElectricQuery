import { createApp } from 'vue'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'

import App from './App.vue'
import router from './router/index.js'

// Material Design 3 主题配色
const vuetify = createVuetify({
  components,
  directives,
  theme: {
    defaultTheme: 'light',
    themes: {
      light: {
        colors: {
          // MD3 Primary (Blue)
          primary:      '#1565C0',
          'on-primary': '#FFFFFF',
          'primary-container':  '#D1E4FF',
          'on-primary-container': '#001D36',
          // Secondary
          secondary:    '#535F70',
          'on-secondary': '#FFFFFF',
          'secondary-container': '#D7E3F7',
          'on-secondary-container': '#101C2B',
          // Tertiary / Accent
          tertiary:     '#6B5778',
          'tertiary-container': '#F2DAFF',
          'on-tertiary-container': '#251431',
          // Surface & Background
          surface:      '#FDFCFF',
          'on-surface':  '#1A1C1E',
          'surface-variant': '#DFE2EB',
          'on-surface-variant': '#43474E',
          'surface-tint': '#1565C0',   // MD3 surface tint
          background:   '#FDFCFF',
          'on-background': '#1A1C1E',
          // Semantic
          error:        '#BA1A1A',
          'on-error':   '#FFFFFF',
          'error-container': '#FFDAD6',
          'on-error-container': '#410002',
          warning:      '#7D5712',
          info:         '#0061A4',
          success:      '#116B29',
          // Outline (MD3 border color)
          outline:      '#73777F',
          'outline-variant': '#C3C7CF',
          // Inverse
          'inverse-surface': '#2F3033',
          'inverse-on-surface': '#F1F0F4',
          'inverse-primary': '#9ECAFF',
          // Scrim
          scrim:        '#000000',
        }
      },
      // 暗色主题（可选）
      dark: {
        colors: {
          primary:      '#9ECAFF',
          'on-primary':  '#003258',
          'primary-container': '#00497D',
          'on-primary-container': '#D1E4FF',
          surface:      '#1A1C1E',
          'on-surface':  '#E3E2E6',
          background:   '#1A1C1E',
          'on-background': '#E3E2E6',
          error:        '#FFB4AB',
          'error-container': '#93000A',
        }
      }
    }
  },
  defaults: {
    // MD3 圆润设计语言
    VBtn: {
      rounded: 'lg',
      style: 'text-transform: none; letter-spacing: 0; font-weight: 500;',
    },
    VCard: {
      rounded: 'xl',
      elevation: 0,       // MD3 用 surface-tint 代替 elevation
      style: 'border: 1px solid rgba(0,0,0,0.08);',
    },
    VTextField: {
      variant: 'outlined',
      density: 'comfortable',
      rounded: 'lg',
    },
    VSelect: {
      variant: 'outlined',
      density: 'comfortable',
      rounded: 'lg',
    },
    VChip: {
      rounded: 'lg',
    },
    VList: {
      rounded: 'lg',
    },
    VAlert: {
      rounded: 'lg',
    },
    VSnackbar: {
      rounded: 'lg',
    },
  }
})

const app = createApp(App)
app.use(router)
app.use(vuetify)
app.mount('#app')
