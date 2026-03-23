---
layout: home

hero:
  name: SuperBotGo SDK
  text: SDK для WASM-плагинов
  tagline: Создавайте изолированные плагины для бот-платформы SuperBotGo на Go и WebAssembly
  actions:
    - theme: brand
      text: Быстрый старт
      link: /guide/quick-start
    - theme: alt
      text: Справочник API
      link: /api/reference

features:
  - icon: 🔒
    title: WASM-песочница
    details: Плагины работают в изолированном WebAssembly — без доступа к файловой системе и сети. Полная изоляция.
  - icon: 🔌
    title: Host API
    details: База данных, HTTP-запросы, KV Store, межплагинные вызовы — всё через функции хоста.
  - icon: ⚡
    title: Мульти-триггеры
    details: Команды мессенджера, HTTP-эндпоинты, Cron-расписания, подписки на события — из одного плагина.
  - icon: 🌳
    title: Node Builder
    details: Ветвление, пагинация, динамические опции, декларативные условия — мощный DSL для команд.
  - icon: 📦
    title: Один бинарник
    details: Один .wasm файл — это весь плагин. Никаких зависимостей в рантайме. Мгновенный hot-reload.
  - icon: 🔄
    title: Миграции
    details: Встроенная поддержка миграций при обновлении версии с доступом к KV Store.
---
