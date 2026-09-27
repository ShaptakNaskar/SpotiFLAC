import i18n, { type BackendModule } from "i18next";
import { initReactI18next } from "react-i18next";
import en from "@/locales/en.json";
export type AppLanguage = "en" | "id" | "nl" | "de" | "es-419" | "fr" | "it" | "pt-BR" | "ru" | "tr" | "vi";
export const APP_LANGUAGES: Array<{
    value: AppLanguage;
    label: string;
    flag: string;
}> = [
    { value: "en", label: "English", flag: "gb" },
    { value: "id", label: "Bahasa Indonesia", flag: "id" },
    { value: "nl", label: "Nederlands", flag: "nl" },
    { value: "de", label: "Deutsch", flag: "de" },
    { value: "es-419", label: "Español (Latinoamérica)", flag: "mx" },
    { value: "fr", label: "Français", flag: "fr" },
    { value: "it", label: "Italiano", flag: "it" },
    { value: "pt-BR", label: "Português (Brasil)", flag: "br" },
    { value: "ru", label: "Русский", flag: "ru" },
    { value: "tr", label: "Türkçe", flag: "tr" },
    { value: "vi", label: "Tiếng Việt", flag: "vn" },
];
export function isAppLanguage(value: unknown): value is AppLanguage {
    return APP_LANGUAGES.some((language) => language.value === value);
}
function flattenMessages(value: unknown, prefix = ""): Array<[
    string,
    string
]> {
    if (typeof value === "string")
        return [[prefix, value]];
    if (!value || typeof value !== "object")
        return [];
    return Object.entries(value).flatMap(([key, child]) => flattenMessages(child, prefix ? `${prefix}.${key}` : key));
}
export const englishMessageKeys = new Map(flattenMessages(en).map(([key, value]) => [value, key]));
const messagePlaceholderPattern = /{{\s*([^{}]+?)\s*}}/g;
const escapeRegExp = (value: string) => value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
const englishMessageTemplates = flattenMessages(en).flatMap(([key, value]) => {
    const matches = [...value.matchAll(messagePlaceholderPattern)];
    if (matches.length === 0 || value.replace(messagePlaceholderPattern, "").replace(/\s/g, "").length < 4) {
        return [];
    }
    let cursor = 0;
    let source = "^";
    const placeholders: string[] = [];
    for (const match of matches) {
        const index = match.index ?? 0;
        source += escapeRegExp(value.slice(cursor, index));
        source += "([\\s\\S]+?)";
        placeholders.push(match[1]);
        cursor = index + match[0].length;
    }
    source += escapeRegExp(value.slice(cursor)) + "$";
    return [{ key, placeholders, pattern: new RegExp(source) }];
});
const localeLoaders = import.meta.glob<{
    default: Record<string, unknown>;
}>(["./locales/*.json", "!./locales/en.json"]);
const lazyLocaleBackend: BackendModule = {
    type: "backend",
    init() { },
    read(language, _namespace, callback) {
        const load = localeLoaders[`./locales/${language}.json`];
        if (!load) {
            callback(new Error(`No translations for ${language}`), false);
            return;
        }
        load().then((module) => callback(null, module.default), (error) => callback(error, false));
    },
};
void i18n.use(lazyLocaleBackend).use(initReactI18next).init({
    resources: {
        en: { translation: en },
    },
    partialBundledLanguages: true,
    lng: "en",
    fallbackLng: "en",
    supportedLngs: APP_LANGUAGES.map((language) => language.value),
    load: "currentOnly",
    interpolation: {
        escapeValue: false,
    },
});
export const t = i18n.t.bind(i18n);
export function translateMessage(message: string): string {
    const key = englishMessageKeys.get(message);
    if (key) {
        return t(key);
    }
    for (const template of englishMessageTemplates) {
        const match = template.pattern.exec(message);
        if (!match) {
            continue;
        }
        const values = Object.fromEntries(template.placeholders.map((placeholder, index) => [placeholder, match[index + 1]]));
        return t(template.key, values);
    }
    return message;
}
export default i18n;
