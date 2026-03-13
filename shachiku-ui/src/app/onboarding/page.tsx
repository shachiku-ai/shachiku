"use client"

import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { API_URL } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Loader2Icon, Wand2Icon, CheckCircle2Icon, ArrowRightIcon, SkipForwardIcon } from "lucide-react"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Logo } from "@/components/logo"

type ConfigState = {
    provider: string
    model: string
    openai_api_key: string
    anthropic_api_key: string
    gemini_api_key: string
    local_api_key: string
    local_endpoint: string
    telegram_bot_token: string
    allowed_telegram_users: string
    ai_name: string
    ai_personality: string
    ai_role: string
    ai_language: string
    ai_soul: string
    setup_completed: boolean
}

const PROVIDERS = [
    { id: "openai", name: "OpenAI" },
    { id: "claude", name: "Anthropic Claude" },
    { id: "gemini", name: "Google Gemini" },
    { id: "local", name: "Local LLM" },
    { id: "claudecode", name: "Claude Code CLI" },
    { id: "geminicli", name: "Gemini CLI" },
    { id: "codexcli", name: "Codex CLI" },
]

export default function OnboardingPage() {
    const { t, i18n } = useTranslation()
    const [step, setStep] = useState(1)
    const [loading, setLoading] = useState(false)
    const [generating, setGenerating] = useState(false)
    const [models, setModels] = useState<string[]>([])
    const [alertMessage, setAlertMessage] = useState<React.ReactNode | null>(null)

    const [config, setConfig] = useState<ConfigState>({
        provider: "openai",
        model: "",
        openai_api_key: "",
        anthropic_api_key: "",
        gemini_api_key: "",
        local_api_key: "",
        local_endpoint: "",
        telegram_bot_token: "",
        allowed_telegram_users: "",
        ai_name: "",
        ai_personality: "",
        ai_role: "",
        ai_language: "",
        ai_soul: "",
        setup_completed: false,
    })

    // eslint-disable-next-line react-hooks/exhaustive-deps
    useEffect(() => {
        fetch(`${API_URL}/config`)
            .then(res => res.json())
            .then(data => {
                if (data) {
                    setConfig(prev => ({ ...prev, ...data }))
                    if (data.provider) fetchModels(data.provider, getApiKeyForProvider(data.provider, data))
                }
            })
            .catch(console.error)
    }, [])

    const getApiKeyForProvider = (providerId: string, cfg: ConfigState = config) => {
        switch (providerId) {
            case "openai": return cfg.openai_api_key
            case "claude": return cfg.anthropic_api_key
            case "gemini": return cfg.gemini_api_key
            case "local": return cfg.local_api_key
            default: return ""
        }
    }

    const setApiKeyForProvider = (providerId: string, value: string) => {
        switch (providerId) {
            case "openai": setConfig(prev => ({ ...prev, openai_api_key: value })); break
            case "claude": setConfig(prev => ({ ...prev, anthropic_api_key: value })); break
            case "gemini": setConfig(prev => ({ ...prev, gemini_api_key: value })); break
            case "local": setConfig(prev => ({ ...prev, local_api_key: value })); break
        }
    }

    const fetchModels = async (providerOverride?: string, apiKeyOverride?: string) => {
        const providerToUse = providerOverride || config.provider
        const apiKeyToUse = apiKeyOverride !== undefined ? apiKeyOverride : getApiKeyForProvider(providerToUse)

        if (["claudecode", "geminicli", "codexcli"].includes(providerToUse)) {
            setModels([`${providerToUse}-local`])
            setConfig(prev => ({ ...prev, model: `${providerToUse}-local` }))
            return
        }

        if (!apiKeyToUse && providerToUse !== "local") {
            setModels([])
            return
        }

        setLoading(true)
        try {
            const res = await fetch(`${API_URL}/models`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ provider: providerToUse, api_key: apiKeyToUse })
            })
            const data = await res.json()
            if (data.models && data.models.length > 0) {
                setModels(data.models)
                if (!config.model || !data.models.includes(config.model)) {
                    setConfig(prev => ({ ...prev, model: data.models[0] }))
                }
            } else {
                setModels([])
            }
        } catch (err) {
            console.error(err)
            setModels([])
        } finally {
            setLoading(false)
        }
    }

    const handleNextStep1 = async () => {
        setLoading(true)
        try {
            const providerToUse = config.provider
            const apiKeyToUse = getApiKeyForProvider(providerToUse)

            const res = await fetch(`${API_URL}/models`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ provider: providerToUse, api_key: apiKeyToUse })
            })
            const data = await res.json()
            if (res.ok && data.models && data.models.length > 0) {
                setStep(2)
            } else {
                if (data.error && data.error.includes("CLI_NOT_INSTALLED")) {
                    setAlertMessage(
                        <span className="flex flex-col gap-2 pt-2">
                            <span>{t("onboarding.cliNotFound", "The required CLI tool is not installed. Please install it to use this provider.")}</span>
                            <a href="https://shachiku.ai/document" target="_blank" rel="noreferrer" className="text-primary hover:underline font-medium break-all mt-1">
                                https://shachiku.ai/document
                            </a>
                        </span>
                    )
                } else {
                    setAlertMessage(t("onboarding.providerVerificationFailed", "Provider verification failed: ") + (data.error || "Unknown Error"))
                }
            }
        } catch (err: unknown) {
            console.error(err)
            const msg = err instanceof Error ? err.message : String(err)
            setAlertMessage(t("onboarding.providerVerificationError", "Error verifying provider: ") + msg)
        } finally {
            setLoading(false)
        }
    }

    const generateSoul = async () => {
        if (!config.ai_name || !config.ai_personality || !config.ai_role) return
        setGenerating(true)
        try {
            await fetch(`${API_URL}/config`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(config)
            })

            const res = await fetch(`${API_URL}/generate-soul`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    name: config.ai_name,
                    personality: config.ai_personality,
                    role: config.ai_role,
                    language: config.ai_language
                })
            })
            const data = await res.json()
            if (res.ok && data.soul) {
                setConfig(prev => ({ ...prev, ai_soul: data.soul }))
                setStep(4)
            } else {
                setAlertMessage(t("onboarding.errorGenerating", "Failed to generate: ") + (data.error || t("onboarding.unknownError", "Unknown error")))
            }
        } catch (e) {
            console.error(e)
            setAlertMessage(t("onboarding.errorGeneratingSoul", "Error generating soul. Please check your connection."))
        } finally {
            setGenerating(false)
        }
    }

    const completeSetup = async (skipTelegram = false) => {
        const finalConfig = { ...config, setup_completed: true }
        if (skipTelegram) {
            finalConfig.telegram_bot_token = ""
        }

        setLoading(true)
        try {
            await fetch(`${API_URL}/config`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(finalConfig)
            })
            // Force a full refresh to ensure all global states re-evaluate
            window.location.href = "/"
        } catch (e) {
            console.error(e)
            setAlertMessage(t("onboarding.errorSavingConfig", "Failed to save configuration."))
        } finally {
            setLoading(false)
        }
    }

    const renderStepIcon = (s: number, current: number) => {
        if (s < current) return <CheckCircle2Icon className="h-5 w-5 text-primary" />
        if (s === current) return <div className="h-5 w-5 rounded-full border-2 border-primary bg-primary/20 flex items-center justify-center text-[10px] font-bold text-primary">{s}</div>
        return <div className="h-5 w-5 rounded-full border-2 border-muted flex items-center justify-center text-[10px] text-muted-foreground">{s}</div>
    }

    return (
        <div className="min-h-screen bg-muted/30 flex flex-col pt-16 sm:px-6 lg:px-8 w-full p-4 relative">
            <div className="absolute top-4 right-4 sm:top-6 sm:right-8 z-50">
                <select
                    className="border border-input rounded-md px-3 py-1.5 text-sm bg-background/50 backdrop-blur-sm cursor-pointer hover:bg-background/80 transition-colors"
                    value={i18n.resolvedLanguage || "en"}
                    onChange={e => i18n.changeLanguage(e.target.value)}
                >
                    <option value="en">English</option>
                    <option value="zh">中文</option>
                    <option value="ja">日本語</option>
                </select>
            </div>
            <div className="w-full max-w-3xl mx-auto">
                <div className="mb-8 text-center">
                    <Logo className="size-30! mx-auto mb-4" />
                    <h1 className="text-3xl font-extrabold tracking-tight">{t("onboarding.title", "Agent Genesis Core")}</h1>
                    <p className="mt-2 text-muted-foreground">{t("onboarding.desc", "Setup and personalize your autonomous AI agent companion")}</p>
                </div>

                <div className="flex items-center justify-between mb-8 relative w-full px-4">
                    <div className="absolute left-8 right-8 top-1/2 -z-10 h-0.5 -translate-y-1/2 bg-muted"></div>
                    <div className="absolute left-8 right-8 top-1/2 -z-10 h-0.5 -translate-y-1/2 bg-primary transition-all duration-500" style={{ right: `calc(100% - ${(step - 1) / 4 * 100}% - 2rem)`, width: `${(step - 1) / 4 * 100}%` }}></div>
                    {[
                        { num: 1, label: t("onboarding.step1", "Model") },
                        { num: 2, label: t("onboarding.step2", "Identity") },
                        { num: 3, label: t("onboarding.step3", "Soul") },
                        { num: 4, label: t("onboarding.step4", "Telegram") },
                        { num: 5, label: t("onboarding.step5", "Finish") }
                    ].map(s => (
                        <div key={s.num} className="flex flex-col items-center pb-1 relative z-10 bg-muted/30 px-2 rounded-md">
                            <div className="bg-background rounded-full p-1 mb-2">
                                {renderStepIcon(s.num, step)}
                            </div>
                            <span className={`text-xs font-medium ${step >= s.num ? 'text-primary' : 'text-muted-foreground'}`}>{s.label}</span>
                        </div>
                    ))}
                </div>

                <Card className="border-0 shadow-lg ring-1 ring-border">
                    {step === 1 && (
                        <>
                            <CardHeader>
                                <CardTitle>{t("onboarding.step1Title", "1. Configure Intelligence Provider")}</CardTitle>
                                <CardDescription>{t("onboarding.step1Desc", "Select the underlying LLM engine for your agent")}</CardDescription>
                            </CardHeader>
                            <CardContent className="space-y-6">
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">{t("onboarding.provider", "Provider")}</label>
                                    <select
                                        className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background md:text-sm"
                                        value={config.provider}
                                        onChange={(e) => {
                                            setConfig(prev => ({ ...prev, provider: e.target.value }))
                                            fetchModels(e.target.value)
                                        }}
                                    >
                                        {PROVIDERS.map(p => (
                                            <option key={p.id} value={p.id}>{p.name}</option>
                                        ))}
                                    </select>
                                </div>

                                {!["claudecode", "geminicli", "codexcli"].includes(config.provider) && (
                                    <div className="space-y-2">
                                        <label className="text-sm font-medium">{t("onboarding.apiKey", "API Key")}</label>
                                        <div className="flex gap-2 w-full flex-col sm:flex-row">
                                            <Input
                                                type="password"
                                                placeholder={`Enter ${PROVIDERS.find(p => p.id === config.provider)?.name} API Key`}
                                                value={getApiKeyForProvider(config.provider)}
                                                onChange={(e) => setApiKeyForProvider(config.provider, e.target.value)}
                                                className="flex-1"
                                            />
                                            <Button onClick={() => fetchModels()} variant="secondary" disabled={loading}>
                                                {loading ? <Loader2Icon className="h-4 w-4 animate-spin" /> : t("onboarding.verifyBtn", "Verify & Fetch Models")}
                                            </Button>
                                        </div>
                                    </div>
                                )}

                                {config.provider === "local" && (
                                    <div className="space-y-2">
                                        <label className="text-sm font-medium">{t("onboarding.localEndpoint", "Local Endpoint (Optional)")}</label>
                                        <Input
                                            placeholder="http://localhost:11434/v1"
                                            value={config.local_endpoint}
                                            onChange={(e) => setConfig(prev => ({ ...prev, local_endpoint: e.target.value }))}
                                        />
                                    </div>
                                )}

                                <div className="space-y-2">
                                    <label className="text-sm font-medium">{t("onboarding.model", "Model")}</label>
                                    <select
                                        className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background disabled:cursor-not-allowed disabled:opacity-50"
                                        value={config.model}
                                        onChange={(e) => setConfig(prev => ({ ...prev, model: e.target.value }))}
                                        disabled={models.length === 0}
                                    >
                                        {models.length === 0 ? (
                                            <option value="">{t("onboarding.noModels", "No models available - Verify API Key first")}</option>
                                        ) : (
                                            models.map(m => (
                                                <option key={m} value={m}>{m}</option>
                                            ))
                                        )}
                                    </select>
                                </div>
                            </CardContent>
                            <CardFooter className="flex justify-end border-t p-6">
                                <Button onClick={handleNextStep1} disabled={!config.model || models.length === 0 || loading}>
                                    {loading ? <Loader2Icon className="mr-2 h-4 w-4 animate-spin" /> : null}
                                    {t("onboarding.nextBtn", "Next Step")} <ArrowRightIcon className="ml-2 h-4 w-4" />
                                </Button>
                            </CardFooter>
                        </>
                    )}

                    {step === 2 && (
                        <>
                            <CardHeader>
                                <CardTitle>{t("onboarding.step2Title", "2. Define Identity")}</CardTitle>
                                <CardDescription>{t("onboarding.step2Desc", "Give your agent a unique name, personality, and role.")}</CardDescription>
                            </CardHeader>
                            <CardContent className="space-y-6">
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">{t("onboarding.agentName", "Agent Name")}</label>
                                    <Input
                                        placeholder={t("onboarding.agentNamePh", "e.g. JARVIS, Alice, Bob...")}
                                        value={config.ai_name}
                                        onChange={(e) => setConfig(prev => ({ ...prev, ai_name: e.target.value }))}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">{t("onboarding.personality", "Personality Constraints (性格)")}</label>
                                    <Input
                                        placeholder={t("onboarding.personalityPh", "e.g. Sarcastic, highly logical, friendly...")}
                                        value={config.ai_personality}
                                        onChange={(e) => setConfig(prev => ({ ...prev, ai_personality: e.target.value }))}
                                    />
                                    <p className="text-xs text-muted-foreground">{t("onboarding.personalityDesc", "Describe how the agent should communicate.")}</p>
                                </div>
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">{t("onboarding.role", "Role Positioning (角色定位)")}</label>
                                    <Input
                                        placeholder={t("onboarding.rolePh", "e.g. Senior DevOps, Casual Assistant...")}
                                        value={config.ai_role}
                                        onChange={(e) => setConfig(prev => ({ ...prev, ai_role: e.target.value }))}
                                    />
                                    <p className="text-xs text-muted-foreground">{t("onboarding.roleDesc", "What is the agent's professional or social purpose?")}</p>
                                </div>
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">{t("onboarding.language", "Output Language (输出语言)")}</label>
                                    <Input
                                        placeholder={t("onboarding.languagePh", "e.g. English, Chinese, Japanese...")}
                                        value={config.ai_language}
                                        onChange={(e) => setConfig(prev => ({ ...prev, ai_language: e.target.value }))}
                                    />
                                    <p className="text-xs text-muted-foreground">{t("onboarding.languageDesc", "The language the generated soul should be in.")}</p>
                                </div>
                            </CardContent>
                            <CardFooter className="flex justify-between border-t p-6">
                                <Button variant="outline" onClick={() => setStep(1)}>{t("onboarding.backBtn", "Back")}</Button>
                                <Button
                                    onClick={generateSoul}
                                    disabled={!config.ai_name || !config.ai_personality || !config.ai_role || !config.ai_language || generating}
                                >
                                    {generating ? <Loader2Icon className="mr-2 h-4 w-4 animate-spin" /> : <Wand2Icon className="mr-2 h-4 w-4" />}
                                    {generating ? t("onboarding.generatingSoul", "Generating Soul...") : t("onboarding.generateSoul", "Generate Soul")}
                                </Button>
                            </CardFooter>
                        </>
                    )}

                    {step === 4 && (
                        <>
                            <CardHeader>
                                <CardTitle>{t("onboarding.step3Title", "3. The Agent's Soul")}</CardTitle>
                                <CardDescription>{t("onboarding.step3Desc", "We've generated a comprehensive system prompt based on your inputs. You can modify it if needed.")}</CardDescription>
                            </CardHeader>
                            <CardContent>
                                <textarea
                                    className="flex w-full rounded-md border border-input bg-muted px-3 py-2 text-sm ring-offset-background font-mono min-h-[300px]"
                                    value={config.ai_soul}
                                    onChange={(e) => setConfig(prev => ({ ...prev, ai_soul: e.target.value }))}
                                />
                            </CardContent>
                            <CardFooter className="flex justify-between border-t p-6">
                                <Button variant="outline" onClick={() => setStep(2)}>{t("onboarding.backBtn", "Back")}</Button>
                                <Button onClick={() => setStep(5)}>{t("onboarding.nextBtn", "Next Step")} <ArrowRightIcon className="ml-2 h-4 w-4" /></Button>
                            </CardFooter>
                        </>
                    )}

                    {step === 5 && (
                        <>
                            <CardHeader>
                                <CardTitle>{t("onboarding.step4Title", "4. Extension: Telegram Module")}</CardTitle>
                                <CardDescription>{t("onboarding.step4Desc", "Optionally bind your agent to a Telegram Bot for mobile chat access.")}</CardDescription>
                            </CardHeader>
                            <CardContent className="space-y-6">
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">{t("onboarding.tgToken", "Telegram Bot Token (Optional)")}</label>
                                    <Input
                                        type="password"
                                        placeholder={t("onboarding.tgTokenPh", "123456789:ABCDEF.......")}
                                        value={config.telegram_bot_token}
                                        onChange={(e) => setConfig(prev => ({ ...prev, telegram_bot_token: e.target.value }))}
                                    />
                                    <p className="text-xs text-muted-foreground">{t("onboarding.tgTokenDesc", "Get this from @BotFather on Telegram. You can skip this.")}</p>
                                </div>
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">{t("onboarding.tgUsers", "Allowed Telegram Usernames")}</label>
                                    <Input
                                        placeholder={t("onboarding.tgUsersPh", "e.g. user1,user2 (leave blank to allow all)")}
                                        value={config.allowed_telegram_users}
                                        onChange={(e) => setConfig(prev => ({ ...prev, allowed_telegram_users: e.target.value }))}
                                    />
                                    <p className="text-xs text-muted-foreground">{t("onboarding.tgUsersDesc", "Comma-separated list of Telegram handles to restrict access.")}</p>
                                </div>
                            </CardContent>
                            <CardFooter className="flex flex-col sm:flex-row justify-between border-t p-6 gap-4">
                                <Button variant="outline" onClick={() => setStep(4)} className="w-full sm:w-auto">{t("onboarding.backBtn", "Back")}</Button>
                                <div className="flex flex-col sm:flex-row gap-2 w-full sm:w-auto">
                                    <Button variant="secondary" onClick={() => completeSetup(true)} disabled={loading} className="w-full sm:w-auto">
                                        <SkipForwardIcon className="mr-2 h-4 w-4" /> {t("onboarding.skipTg", "Skip Telegram")}
                                    </Button>
                                    <Button onClick={() => completeSetup(false)} disabled={loading} className="w-full sm:w-auto">
                                        {loading && <Loader2Icon className="mr-2 h-4 w-4 animate-spin" />}
                                        {t("onboarding.completeSetup", "Complete Setup")}
                                    </Button>
                                </div>
                            </CardFooter>
                        </>
                    )}

                </Card>
            </div>

            <Dialog open={!!alertMessage} onOpenChange={(open) => !open && setAlertMessage(null)}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("onboarding.alertTitle", "Notification")}</DialogTitle>
                        <DialogDescription className="text-base text-foreground mt-2">
                            {alertMessage}
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button onClick={() => setAlertMessage(null)}>{t("onboarding.closeBtn", "Close")}</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
