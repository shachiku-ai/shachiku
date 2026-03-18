"use client"

import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { API_URL } from "@/lib/api"
import { getProviderErrorMsg } from "@/lib/utils"
import { SiteHeader } from "@/components/site-header"
import { Check, Loader2Icon } from "lucide-react"
import { useTheme } from "next-themes"

export default function SettingsPage() {
    const { t, i18n } = useTranslation()
    const { theme, setTheme } = useTheme()
    const [config, setConfig] = useState({
        provider: "openai",
        model: "",
        openai_api_key: "",
        anthropic_api_key: "",
        gemini_api_key: "",
        openrouter_api_key: "",
        local_api_key: "",
        local_endpoint: "",
        openaicompatible_api_key: "",
        openaicompatible_endpoint: "",
        telegram_bot_token: "",
        allowed_telegram_users: "",
        discord_bot_token: "",
        allowed_discord_users: "",
        channel_provider: "telegram",
        ai_name: "",
        ai_personality: "",
        ai_role: "",
        ai_soul: "",
        setup_completed: true,
        max_iterations: 50
    })

    const [step, setStep] = useState(1)
    const [status, setStatus] = useState<React.ReactNode | null>(null)
    const [isFetching, setIsFetching] = useState(false)
    const [modelsList, setModelsList] = useState<string[]>([])
    const [activeTab, setActiveTab] = useState("llm")
    const [isSaving, setIsSaving] = useState(false)
    const [isSaved, setIsSaved] = useState(false)
    const [isDataLoaded, setIsDataLoaded] = useState(false)

    useEffect(() => {
        fetch(`${API_URL}/config`)
            .then(res => res.json())
            .then(data => {
                setConfig({
                    provider: data.provider || "openai",
                    model: data.model || "",
                    openai_api_key: data.openai_api_key || "",
                    anthropic_api_key: data.anthropic_api_key || "",
                    gemini_api_key: data.gemini_api_key || "",
                    openrouter_api_key: data.openrouter_api_key || "",
                    local_api_key: data.local_api_key || "",
                    local_endpoint: data.local_endpoint || "",
                    openaicompatible_api_key: data.openaicompatible_api_key || "",
                    openaicompatible_endpoint: data.openaicompatible_endpoint || "",
                    telegram_bot_token: data.telegram_bot_token || "",
                    allowed_telegram_users: data.allowed_telegram_users || "",
                    discord_bot_token: data.discord_bot_token || "",
                    allowed_discord_users: data.allowed_discord_users || "",
                    channel_provider: data.channel_provider || "telegram",
                    ai_name: data.ai_name || "",
                    ai_personality: data.ai_personality || "",
                    ai_role: data.ai_role || "",
                    ai_soul: data.ai_soul || "",
                    setup_completed: data.setup_completed ?? true,
                    max_iterations: data.max_iterations || 50
                })
            })
            .catch(err => console.error("Failed to load config", err))
            .finally(() => setIsDataLoaded(true))
    }, [])

    const getActiveKey = () => {
        if (config.provider === "claude") return config.anthropic_api_key
        if (config.provider === "gemini") return config.gemini_api_key
        if (config.provider === "openrouter") return config.openrouter_api_key
        if (config.provider === "openaicompatible") return config.openaicompatible_api_key
        return config.openai_api_key
    }

    const setActiveKey = (val: string) => {
        if (config.provider === "claude") {
            setConfig({ ...config, anthropic_api_key: val })
        } else if (config.provider === "gemini") {
            setConfig({ ...config, gemini_api_key: val })
        } else if (config.provider === "openrouter") {
            setConfig({ ...config, openrouter_api_key: val })
        } else if (config.provider === "openaicompatible") {
            setConfig({ ...config, openaicompatible_api_key: val })
        } else {
            setConfig({ ...config, openai_api_key: val })
        }
    }

    const handleNext = async () => {
        const apiKey = getActiveKey()
        if (!["claudecode", "geminicli", "codexcli", "local", "openaicompatible"].includes(config.provider) && !apiKey) {
            setStatus(t("settings.apiKeyRequired", "API Key is required"))
            return
        }

        setStatus("")
        setIsFetching(true)

        try {
            const res = await fetch(`${API_URL}/models`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    provider: config.provider,
                    api_key: apiKey,
                    endpoint: config.provider === "openaicompatible" ? config.openaicompatible_endpoint : (config.provider === "local" ? config.local_endpoint : "")
                })
            })

            const data = await res.json()
            if (!res.ok) {
                if (data.error && data.error.includes("CLI_NOT_INSTALLED")) {
                    setStatus(
                        <span className="flex flex-col gap-1 text-left">
                            <span>{t("settings.cliNotFound", "The required CLI tool is not installed.")}</span>
                            <a href="https://shachiku.ai/document" target="_blank" rel="noreferrer" className="text-primary hover:underline font-medium break-all">
                                https://shachiku.ai/document
                            </a>
                        </span>
                    )
                    return
                }
                throw new Error(data.error || "Failed to fetch models")
            }

            setModelsList(data.models || [])
            if (data.models && data.models.length > 0 && !data.models.includes(config.model)) {
                // If current model isn't in fetched list, default to first
                setConfig(prev => ({ ...prev, model: data.models[0] }))
            }

            setStep(2)
        } catch (err: unknown) {
            console.error(err)
            if (err instanceof Error) {
                setStatus(getProviderErrorMsg(t, err.message) || t("settings.failedVerify", "Failed to verify API Key"))
            } else {
                setStatus(t("settings.failedVerify", "Failed to verify API Key"))
            }
        } finally {
            setIsFetching(false)
        }
    }

    const handleSave = async () => {
        setIsSaving(true)
        setStatus("")
        try {
            const res = await fetch(`${API_URL}/config`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(config)
            })
            if (!res.ok) throw new Error(t("settings.errorSaving", "Failed to save"))
            setIsSaved(true)
            setTimeout(() => setIsSaved(false), 2000)
        } catch (err) {
            console.error(err)
            setStatus(t("settings.errorSaving", "Error saving"))
        } finally {
            setIsSaving(false)
        }
    }

    return (
        <>
            <SiteHeader title={t("settings.title", "Settings")} />
            <div className="flex flex-1 flex-col p-4 lg:p-6 pb-20 mx-auto w-full max-w-6xl">
                <div className="mb-8 border-b pb-6">
                    <h2 className="text-3xl font-bold tracking-tight">{t("settings.systemSettings", "System Settings")}</h2>
                    <p className="text-muted-foreground mt-2">
                        {t("settings.systemSettingsDesc", "Configure LLM provider, validate API key, and select model.")}
                    </p>
                </div>

                {!isDataLoaded ? (
                    <div className="flex items-center justify-center p-20">
                        <Loader2Icon className="h-8 w-8 animate-spin text-muted-foreground" />
                    </div>
                ) : (
                    <div className="flex flex-col md:flex-row gap-8 w-full">
                        <aside className="w-full md:w-64 shrink-0">
                            <nav className="flex flex-col gap-1">
                                <button
                                    onClick={() => { setActiveTab("llm"); setStatus(""); setIsSaved(false); }}
                                    className={`text-left px-4 py-2.5 rounded-md font-medium text-sm transition-colors ${activeTab === "llm" ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground hover:bg-muted/50"}`}
                                >
                                    {t("settings.llmConfig", "LLM Configuration")}
                                </button>
                                <button
                                    onClick={() => { setActiveTab("identity"); setStatus(""); setIsSaved(false); }}
                                    className={`text-left px-4 py-2.5 rounded-md font-medium text-sm transition-colors ${activeTab === "identity" ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground hover:bg-muted/50"}`}
                                >
                                    {t("settings.agentIdentity", "Agent Identity (Soul)")}
                                </button>
                                <button
                                    onClick={() => { setActiveTab("agent"); setStatus(""); setIsSaved(false); }}
                                    className={`text-left px-4 py-2.5 rounded-md font-medium text-sm transition-colors ${activeTab === "agent" ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground hover:bg-muted/50"}`}
                                >
                                    {t("settings.agentSettings", "Agent Settings")}
                                </button>
                                <button
                                    onClick={() => { setActiveTab("channel"); setStatus(""); setIsSaved(false); }}
                                    className={`text-left px-4 py-2.5 rounded-md font-medium text-sm transition-colors ${activeTab === "channel" ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground hover:bg-muted/50"}`}
                                >
                                    {t("settings.channelSettings", "External Channel")}
                                </button>
                            </nav>
                        </aside>

                        <div className="flex-1 max-w-3xl">
                            {activeTab === "llm" && (
                                <div className="flex flex-col gap-6 w-full rounded-lg border shadow-sm p-6 relative">
                                    <div className="absolute top-4 right-6 text-sm text-muted-foreground font-medium">
                                        {t("settings.stepOf2", "Step {{step}} of 2", { step })}
                                    </div>
                                    <h3 className="text-xl font-bold">{t("settings.llmConfig", "LLM Configuration")}</h3>

                                    {step === 1 && (
                                        <>
                                            <div className="grid gap-2">
                                                <label className="text-sm font-medium">{t("settings.provider", "Provider")}</label>
                                                <select
                                                    className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                                    value={config.provider}
                                                    onChange={e => setConfig({ ...config, provider: e.target.value })}
                                                >
                                                    <option value="openai">{t("settings.openai", "OpenAI")}</option>
                                                    <option value="claude">{t("settings.claude", "Anthropic (Claude)")}</option>
                                                    <option value="gemini">{t("settings.gemini", "Google (Gemini)")}</option>
                                                    <option value="openrouter">{t("settings.openrouter", "OpenRouter")}</option>
                                                    <option value="claudecode">{t("settings.claudecode", "Local (Claude Code)")}</option>
                                                    <option value="geminicli">{t("settings.geminicli", "Local (Gemini CLI)")}</option>
                                                    <option value="codexcli">{t("settings.codexcli", "Local (Codex CLI)")}</option>
                                                    <option value="openaicompatible">{t("settings.openaicompatible", "OpenAI Compatible")}</option>
                                                    <option value="local">{t("settings.local", "Local (Ollama/Legacy)")}</option>
                                                </select>
                                            </div>

                                            {!["claudecode", "geminicli", "codexcli", "local"].includes(config.provider) && (
                                                <div className="grid gap-4 mt-2">
                                                    {(config.provider === "openaicompatible") && (
                                                        <div className="grid gap-2">
                                                            <label className="text-sm font-medium">{t("settings.endpoint", "API Endpoint")}</label>
                                                            <input
                                                                type="text"
                                                                placeholder={t("settings.endpointPlaceholder", "e.g. https://api.moonshot.cn/v1")}
                                                                className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                                                value={config.openaicompatible_endpoint}
                                                                onChange={e => setConfig({ ...config, openaicompatible_endpoint: e.target.value })}
                                                            />
                                                        </div>
                                                    )}
                                                    <div className="grid gap-2">
                                                        <label className="text-sm font-medium">{t("settings.apiKey", "API Key")}</label>
                                                        <input
                                                            type="password"
                                                            placeholder={config.provider === "openaicompatible" ? t("settings.apiKeyOptionalPlaceholder", "Enter API Key (Optional)...") : t("settings.apiKeyPlaceholder", "Enter your API Key...")}
                                                            className="border rounded-md px-3 py-2 text-sm bg-transparent font-mono disabled:opacity-50"
                                                            value={getActiveKey()}
                                                            onChange={e => setActiveKey(e.target.value)}
                                                            onKeyDown={e => {
                                                                if (e.key === "Enter") handleNext()
                                                            }}
                                                        />
                                                        <p className="text-xs text-muted-foreground">
                                                            {t("settings.apiKeyVerifyText", "Your key will be verified in the next step to fetch available models.")}
                                                        </p>
                                                    </div>
                                                </div>
                                            )}

                                            <div className="flex items-center justify-between mt-4">
                                                <button
                                                    onClick={handleNext}
                                                    disabled={isFetching}
                                                    className="bg-black dark:bg-white text-white dark:text-black py-2 px-4 rounded-md text-sm font-medium hover:opacity-90 transition-opacity disabled:opacity-50"
                                                >
                                                    {isFetching ? t("settings.verifying", "Verifying...") : t("settings.next", "Next ->")}
                                                </button>
                                                {status && <div className="text-sm text-red-500 flex-1 ml-4">{status}</div>}
                                            </div>
                                        </>
                                    )}

                                    {step === 2 && (
                                        <>
                                            <div className="grid gap-2">
                                                <label className="text-sm font-medium min-w-max">{t("settings.provider", "Provider")}</label>
                                                <div className="px-3 py-2 border rounded-md bg-muted/30 text-sm font-medium uppercase tracking-wider text-muted-foreground cursor-not-allowed">
                                                    {config.provider}
                                                </div>
                                            </div>

                                            <div className="grid gap-2">
                                                <label className="text-sm font-medium">{t("settings.selectModel", "Select Model")}</label>
                                                <select
                                                    className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                                    value={config.model}
                                                    onChange={e => setConfig({ ...config, model: e.target.value })}
                                                >
                                                    {modelsList.map(m => (
                                                        <option key={m} value={m}>{m}</option>
                                                    ))}
                                                </select>
                                            </div>

                                            <div className="flex items-center justify-between mt-4 gap-2">
                                                <button
                                                    onClick={() => {
                                                        setStep(1);
                                                        setStatus("");
                                                    }}
                                                    className="border py-2 px-4 rounded-md text-sm font-medium hover:bg-muted transition-colors"
                                                >
                                                    {t("settings.back", "Back")}
                                                </button>
                                                <button
                                                    onClick={handleSave}
                                                    disabled={isSaving || isSaved}
                                                    className="bg-black dark:bg-white text-white dark:text-black py-2 px-4 rounded-md text-sm font-medium hover:opacity-90 transition-opacity flex-1 flex items-center justify-center disabled:opacity-50 min-h-[40px]"
                                                >
                                                    {isSaved ? <Check className="w-5 h-5" /> : (isSaving ? t("settings.saving", "Saving...") : t("settings.saveSettings", "Save Settings"))}
                                                </button>
                                            </div>
                                            {status && <p className="text-sm text-center text-red-500 mt-2">{status}</p>}
                                        </>
                                    )}
                                </div>
                            )}

                            {activeTab === "identity" && (
                                <div className="flex flex-col gap-6 w-full rounded-lg border shadow-sm p-6 relative">
                                    <h3 className="text-xl font-bold">{t("settings.agentIdentity", "Agent Identity (Soul)")}</h3>

                                    <div className="grid gap-2">
                                        <label className="text-sm font-medium">{t("settings.agentName", "Agent Name")}</label>
                                        <input
                                            type="text"
                                            className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                            value={config.ai_name}
                                            onChange={e => setConfig({ ...config, ai_name: e.target.value })}
                                        />
                                    </div>

                                    <div className="grid gap-2">
                                        <label className="text-sm font-medium">{t("settings.personality", "Personality")}</label>
                                        <input
                                            type="text"
                                            className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                            value={config.ai_personality}
                                            onChange={e => setConfig({ ...config, ai_personality: e.target.value })}
                                        />
                                    </div>

                                    <div className="grid gap-2">
                                        <label className="text-sm font-medium">{t("settings.role", "Role")}</label>
                                        <input
                                            type="text"
                                            className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                            value={config.ai_role}
                                            onChange={e => setConfig({ ...config, ai_role: e.target.value })}
                                        />
                                    </div>

                                    <div className="grid gap-2">
                                        <label className="text-sm font-medium">{t("settings.soul", "Soul (System Instructions)")}</label>
                                        <textarea
                                            className="border rounded-md px-3 py-2 text-sm bg-transparent min-h-[150px] font-mono"
                                            value={config.ai_soul}
                                            onChange={e => setConfig({ ...config, ai_soul: e.target.value })}
                                        />
                                    </div>

                                    <div className="flex items-center justify-between mt-4">
                                        <span className="text-sm text-red-500">
                                            {status}
                                        </span>
                                        <button
                                            onClick={handleSave}
                                            disabled={isSaving || isSaved}
                                            className="bg-black dark:bg-white text-white dark:text-black py-2 px-4 rounded-md text-sm font-medium hover:opacity-90 transition-opacity flex items-center justify-center min-w-[140px] min-h-[40px] disabled:opacity-50"
                                        >
                                            {isSaved ? <Check className="w-5 h-5" /> : (isSaving ? t("settings.saving", "Saving...") : t("settings.saveIdentity", "Save Identity"))}
                                        </button>
                                    </div>
                                </div>
                            )}

                            {activeTab === "agent" && (
                                <div className="flex flex-col gap-6 w-full rounded-lg border shadow-sm p-6 relative">
                                    <h3 className="text-xl font-bold">{t("settings.agentSettings", "Agent Settings")}</h3>

                                    <div className="grid gap-2">
                                        <label className="text-sm font-medium">{t("settings.language", "UI Language")}</label>
                                        <select
                                            className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                            value={i18n.resolvedLanguage || "en"}
                                            onChange={e => i18n.changeLanguage(e.target.value)}
                                        >
                                            <option value="en">English</option>
                                            <option value="zh">中文 (Chinese)</option>
                                            <option value="ja">日本語 (Japanese)</option>
                                        </select>
                                    </div>

                                    <div className="grid gap-2">
                                        <label className="text-sm font-medium">{t("settings.theme", "Theme")}</label>
                                        <select
                                            className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                            value={theme || "system"}
                                            onChange={e => setTheme(e.target.value)}
                                        >
                                            <option value="system">{t("settings.themeSystem", "System")}</option>
                                            <option value="light">{t("settings.themeLight", "Light")}</option>
                                            <option value="dark">{t("settings.themeDark", "Dark")}</option>
                                        </select>
                                    </div>

                                    <div className="grid gap-2">
                                        <label className="text-sm font-medium">{t("settings.maxThinkingSteps", "Max Thinking Steps")}</label>
                                        <input
                                            type="number"
                                            min="1"
                                            className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                            value={config.max_iterations}
                                            onChange={e => setConfig({ ...config, max_iterations: parseInt(e.target.value) || 50 })}
                                        />
                                        <p className="text-xs text-muted-foreground">
                                            {t("settings.maxThinkingDesc", "Limit the number of background tool executions the agent can make in a single run.")}
                                        </p>
                                    </div>

                                    <div className="flex items-center justify-between mt-4 border-t pt-4">
                                        <span className="text-sm text-red-500">
                                            {status}
                                        </span>
                                        <button
                                            onClick={handleSave}
                                            disabled={isSaving || isSaved}
                                            className="bg-black dark:bg-white text-white dark:text-black py-2 px-4 rounded-md text-sm font-medium hover:opacity-90 transition-opacity flex items-center justify-center min-w-[140px] min-h-[40px] disabled:opacity-50"
                                        >
                                            {isSaved ? <Check className="w-5 h-5" /> : (isSaving ? t("settings.saving", "Saving...") : t("settings.saveSettings", "Save Settings"))}
                                        </button>
                                    </div>
                                </div>
                            )}

                            {activeTab === "channel" && (
                                <div className="flex flex-col gap-6 w-full rounded-lg border shadow-sm p-6 relative">
                                    <h3 className="text-xl font-bold">{t("settings.channelSettings", "External Channel Configuration")}</h3>

                                    <div className="grid gap-2">
                                        <label className="text-sm font-medium">{t("settings.channelProvider", "External Channel Provider")}</label>
                                        <select
                                            className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                            value={config.channel_provider}
                                            onChange={e => setConfig({ ...config, channel_provider: e.target.value })}
                                        >
                                            <option value="none">{t("settings.channelNone", "None (UI Only)")}</option>
                                            <option value="telegram">Telegram</option>
                                            <option value="discord">Discord</option>
                                        </select>
                                    </div>

                                    {config.channel_provider === "telegram" && (
                                        <>
                                            <div className="grid gap-2">
                                                <label className="text-sm font-medium">{t("settings.telegramBotToken", "Telegram Bot Token")}</label>
                                                <input
                                                    type="password"
                                                    placeholder={t("settings.telegramTokenPlaceholder", "Enter Telegram Bot Token (optional)...")}
                                                    className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                                    value={config.telegram_bot_token || ""}
                                                    onChange={e => setConfig({ ...config, telegram_bot_token: e.target.value })}
                                                />
                                                <p className="text-xs text-muted-foreground">
                                                    {t("settings.telegramTokenDesc", "If configured, the agent will reply to messages sent to this Telegram bot.")}
                                                </p>
                                            </div>

                                            <div className="grid gap-2">
                                                <label className="text-sm font-medium">{t("settings.allowedTelegramUsers", "Allowed Telegram Users")}</label>
                                                <input
                                                    type="text"
                                                    placeholder={t("settings.allowedUsersPlaceholder", "alice,bob")}
                                                    className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                                    value={config.allowed_telegram_users || ""}
                                                    onChange={e => setConfig({ ...config, allowed_telegram_users: e.target.value })}
                                                />
                                                <p className="text-xs text-muted-foreground">
                                                    {t("settings.allowedUsersDesc", "Comma-separated list of allowed Telegram usernames. Leave empty to allow any user.")}
                                                </p>
                                            </div>
                                        </>
                                    )}

                                    {config.channel_provider === "discord" && (
                                        <>
                                            <div className="grid gap-2">
                                                <label className="text-sm font-medium">{t("settings.discordBotToken", "Discord Bot Token")}</label>
                                                <input
                                                    type="password"
                                                    placeholder={t("settings.discordTokenPlaceholder", "Enter Discord Bot Token...")}
                                                    className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                                    value={config.discord_bot_token || ""}
                                                    onChange={e => setConfig({ ...config, discord_bot_token: e.target.value })}
                                                />
                                                <p className="text-xs text-muted-foreground">
                                                    {t("settings.discordTokenDesc", "If configured, the agent will reply to messages sent to this Discord bot.")}
                                                </p>
                                            </div>

                                            <div className="grid gap-2">
                                                <label className="text-sm font-medium">{t("settings.allowedDiscordUsers", "Allowed Discord Users/Channels")}</label>
                                                <input
                                                    type="text"
                                                    placeholder={t("settings.allowedDiscordUsersPlaceholder", "alice,bob")}
                                                    className="border rounded-md px-3 py-2 text-sm bg-transparent"
                                                    value={config.allowed_discord_users || ""}
                                                    onChange={e => setConfig({ ...config, allowed_discord_users: e.target.value })}
                                                />
                                                <p className="text-xs text-muted-foreground">
                                                    {t("settings.allowedDiscordUsersDesc", "Comma-separated list of allowed Discord usernames or User IDs. Leave empty to allow any user.")}
                                                </p>
                                            </div>
                                        </>
                                    )}

                                    <div className="flex items-center justify-between mt-4">
                                        <span className="text-sm text-red-500">
                                            {status}
                                        </span>
                                        <button
                                            onClick={handleSave}
                                            disabled={isSaving || isSaved}
                                            className="bg-black dark:bg-white text-white dark:text-black py-2 px-4 rounded-md text-sm font-medium hover:opacity-90 transition-opacity flex items-center justify-center min-w-[140px] min-h-[40px] disabled:opacity-50"
                                        >
                                            {isSaved ? <Check className="w-5 h-5" /> : (isSaving ? t("settings.saving", "Saving...") : t("settings.saveSettings", "Save Settings"))}
                                        </button>
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                )}
            </div>
        </>
    )
}
