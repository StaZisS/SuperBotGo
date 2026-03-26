import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

export default withMermaid(defineConfig({
  title: 'SuperBotGo SDK',
  description: 'Документация по SDK для WASM-плагинов SuperBotGo',
  lang: 'ru-RU',

  themeConfig: {
    logo: '/logo.svg',

    nav: [
      { text: 'Архитектура', link: '/architecture/components' },
      { text: 'Руководство', link: '/guide/quick-start' },
      { text: 'API', link: '/api/context' },
      { text: 'Продвинутое', link: '/advanced/node-builder' },
    ],

    sidebar: {
      '/architecture/': [
        {
          text: 'Архитектура',
          items: [
            { text: 'Диаграмма компонентов', link: '/architecture/components' },
          ],
        },
      ],
      '/guide/': [
        {
          text: 'Начало работы',
          items: [
            { text: 'Быстрый старт', link: '/guide/quick-start' },
            { text: 'Структура плагина', link: '/guide/plugin-structure' },
          ],
        },
        {
          text: 'Основы',
          items: [
            { text: 'Команды', link: '/guide/commands' },
            { text: 'Триггеры', link: '/guide/triggers' },
            { text: 'Конфигурация', link: '/guide/configuration' },
          ],
        },
      ],
      '/api/': [
        {
          text: 'Контекст и API',
          items: [
            { text: 'EventContext', link: '/api/context' },
            { text: 'KV Store', link: '/api/kv-store' },
            { text: 'Уведомления', link: '/api/notifications' },
            { text: 'Host API', link: '/api/host-api' },
            { text: 'Справочник', link: '/api/reference' },
          ],
        },
      ],
      '/advanced/': [
        {
          text: 'Продвинутое',
          items: [
            { text: 'Node Builder', link: '/advanced/node-builder' },
            { text: 'Условия', link: '/advanced/conditions' },
            { text: 'Миграции', link: '/advanced/migrations' },
            { text: 'Сборка и деплой', link: '/advanced/build' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com' },
    ],

    search: {
      provider: 'local',
    },

    outline: {
      level: [2, 3],
      label: 'На этой странице',
    },

    footer: {
      message: 'Документация SuperBotGo Plugin SDK',
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
