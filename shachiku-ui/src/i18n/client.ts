"use client";

import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";

import en from "./locales/en.json";
import zh from "./locales/zh.json";
import ja from "./locales/ja.json";

i18next
    .use(LanguageDetector)
    .use(initReactI18next)
    .init({
        resources: {
            en: { translation: en },
            zh: { translation: zh },
            ja: { translation: ja },
        },
        fallbackLng: "en",
        load: "languageOnly",
        interpolation: {
            escapeValue: false,
        },
    });

export default i18next;
