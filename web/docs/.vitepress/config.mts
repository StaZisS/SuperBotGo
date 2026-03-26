import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

export default withMermaid(defineConfig({
  base: '/SuperBotGo/',
  title: 'SuperBotGo SDK',
  description: 'Документация по SDK для WASM-плагинов SuperBotGo',
  lang: 'ru-RU',

  themeConfig: {
    logo: '/logo.svg',

    nav: [
      { text: 'Руководство', link: '/guide/overview' },
      { text: 'API', link: '/api/context' },
      { text: 'Деплой', link: '/deploy/build' },
      { text: 'Архитектура', link: '/architecture/components' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Начало работы',
          items: [
            { text: 'Обзор', link: '/guide/overview' },
            { text: 'Быстрый старт', link: '/guide/quick-start' },
            { text: 'Структура плагина', link: '/guide/plugin-structure' },
          ],
        },
        {
          text: 'Триггеры',
          items: [
            { text: 'Обзор триггеров', link: '/guide/triggers' },
            { text: 'Messenger', link: '/guide/trigger-messenger' },
            { text: 'HTTP', link: '/guide/trigger-http' },
            { text: 'Cron', link: '/guide/trigger-cron' },
            { text: 'Event Bus', link: '/guide/trigger-event' },
          ],
        },
        {
          text: 'Настройка',
          items: [
            { text: 'Конфигурация', link: '/guide/configuration' },
          ],
        },
      ],
      '/api/': [
        {
          text: 'Контекст',
          items: [
            { text: 'EventContext', link: '/api/context' },
          ],
        },
        {
          text: 'Host API',
          items: [
            { text: 'Обзор', link: '/api/host-api' },
            { text: 'База данных', link: '/api/database' },
            { text: 'HTTP-клиент', link: '/api/http' },
            { text: 'KV Store', link: '/api/kv-store' },
            { text: 'Уведомления', link: '/api/notifications' },
            { text: 'Плагины и события', link: '/api/plugins' },
          ],
        },
        {
          text: 'Прочее',
          items: [
            { text: 'Локализация', link: '/api/localization' },
            { text: 'Справочник', link: '/api/reference' },
          ],
        },
      ],
      '/deploy/': [
        {
          text: 'Деплой',
          items: [
            { text: 'Сборка и установка', link: '/deploy/build' },
            { text: 'Миграции', link: '/deploy/migrations' },
            { text: 'Масштабирование', link: '/deploy/scaling' },
          ],
        },
      ],
      '/architecture/': [
        {
          text: 'Архитектура',
          items: [
            { text: 'Компоненты системы', link: '/architecture/components' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/StaZisS/SuperBotGo' },
    ],

    search: {
      provider: 'local',
    },

    outline: {
      level: [2, 3],
      label: 'На этой странице',
    },

    docFooter: {
      prev: 'Предыдущая',
      next: 'Следующая',
    },

    darkModeSwitchLabel: 'Тема',
    sidebarMenuLabel: 'Меню',
    returnToTopLabel: 'Наверх',

    lastUpdated: {
      text: 'Обновлено',
    },
  },

  mermaid: {
    theme: 'base',
  },
}))
