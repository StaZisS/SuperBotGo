---
layout: home

hero:
  name: Документация SuperBotGo
  text: Платформа, плагины, Host API и деплой
  tagline: Архитектура системы, разработка WASM-плагинов, интеграции, эксплуатация и SDK для разных языков
  actions:
    - theme: brand
      text: Обзор платформы
      link: /guide/overview
    - theme: alt
      text: Быстрый старт (Go)
      link: /guide/quick-start
    - theme: alt
      text: Справочник API
      link: /api/reference

features:
  - title: Платформа
    details: "Как устроен SuperBotGo: каналы, триггеры, плагины, Host API и ключевые системные потоки."
    link: /guide/overview
    linkText: Открыть
  - title: Разработка плагинов
    details: Структура плагина, конфигурация, жизненный цикл и практические сценарии разработки.
    link: /guide/plugin-structure
    linkText: Изучить
  - title: SDK и языки
    details: "Документация описывает всю систему. Примеры быстрого старта сейчас даны для Go SDK, но модель плагинов не ограничена Go."
    link: /guide/quick-start
    linkText: Перейти
  - title: Триггеры и интеграции
    details: Messenger-команды, HTTP-эндпоинты, Cron-расписания и Event Bus для межплагинного обмена.
    link: /guide/triggers
    linkText: Подробнее
  - title: API и Host ABI
    details: "EventContext, Host API, данные, файлы, локализация, уведомления и справочные контракты."
    link: /api/context
    linkText: Справочник
  - title: Архитектура и эксплуатация
    details: Компоненты системы, авторизация, хранение файлов, миграции, сборка и масштабирование.
    link: /architecture/components
    linkText: Смотреть
---
