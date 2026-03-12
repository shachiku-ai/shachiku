"use client"

import { useState } from "react"
import { useTranslation } from "react-i18next"
import { API_URL } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Loader2Icon } from "lucide-react"
import { Logo } from "@/components/logo"

export default function SetupAuthPage() {
    const { t, i18n } = useTranslation()
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState("")

    const [form, setForm] = useState({
        username: "",
        password: "",
        confirm_password: ""
    })

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (form.password !== form.confirm_password) {
            setError(t("setupAuth.pwdMismatch", "Passwords do not match"))
            return
        }

        setError("")
        setLoading(true)

        const fd = new FormData()
        fd.append("username", form.username)
        fd.append("password", form.password)
        fd.append("confirm_password", form.confirm_password)

        try {
            const res = await fetch(`${API_URL}/setup-auth`, {
                method: "POST",
                body: fd
            })

            if (res.ok) {
                window.location.href = "/onboarding"
            } else {
                const text = await res.text()
                setError(text)
            }
        } catch (err) {
            console.error(err)
            setError(t("setupAuth.networkError", "Network error occurred"))
        } finally {
            setLoading(false)
        }
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

            <div className="w-full max-w-md mx-auto mt-10">
                <Card className="border-0 shadow-lg ring-1 ring-border">
                    <CardHeader className="text-center">
                        <div className="flex justify-center mb-6 mt-2">
                            <Logo className="size-20" />
                        </div>
                        <CardTitle className="text-2xl">
                            {t("setupAuth.title", "Admin Setup")}
                        </CardTitle>
                        <CardDescription>
                            {t("setupAuth.desc", "Set up an admin username and password.")}
                        </CardDescription>
                    </CardHeader>

                    <form onSubmit={handleSubmit}>
                        <CardContent className="space-y-4">
                            {error && (
                                <div className="text-sm text-destructive-foreground bg-destructive/10 p-3 rounded-md">
                                    {error}
                                </div>
                            )}

                            <div className="space-y-2">
                                <label className="text-sm font-medium">{t("setupAuth.username", "Username")}</label>
                                <Input
                                    required
                                    value={form.username}
                                    onChange={e => setForm({ ...form, username: e.target.value })}
                                />
                            </div>

                            <div className="space-y-2">
                                <label className="text-sm font-medium">{t("setupAuth.password", "Password")}</label>
                                <Input
                                    type="password"
                                    required
                                    value={form.password}
                                    onChange={e => setForm({ ...form, password: e.target.value })}
                                />
                            </div>

                            <div className="space-y-2 mb-6">
                                <label className="text-sm font-medium">{t("setupAuth.confirmPassword", "Confirm Password")}</label>
                                <Input
                                    type="password"
                                    required
                                    value={form.confirm_password}
                                    onChange={e => setForm({ ...form, confirm_password: e.target.value })}
                                />
                            </div>
                        </CardContent>
                        <CardFooter>
                            <Button type="submit" className="w-full" disabled={loading}>
                                {loading && <Loader2Icon className="mr-2 h-4 w-4 animate-spin" />}
                                {t("setupAuth.submit", "Complete Setup")}
                            </Button>
                        </CardFooter>
                    </form>
                </Card>
            </div>
        </div>
    )
}
