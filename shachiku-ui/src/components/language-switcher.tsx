"use client"

import { useTranslation } from "react-i18next"
import { useEffect, useState } from "react"

export function LanguageSwitcher() {
    const { i18n } = useTranslation()
    const [mounted, setMounted] = useState(false)

    useEffect(() => {
        setMounted(true)
    }, [])

    if (!mounted) {
        return (
            <select className="border border-input rounded-md px-2 py-1.5 text-xs bg-background/50 backdrop-blur-sm cursor-pointer" disabled>
                <option>EN</option>
            </select>
        )
    }

    return (
        <select
            className="border border-input rounded-md px-2 py-1.5 text-xs bg-background/50 backdrop-blur-sm cursor-pointer hover:bg-background/80 transition-colors"
            value={i18n.resolvedLanguage || "en"}
            onChange={e => i18n.changeLanguage(e.target.value)}
        >
            <option value="en">EN</option>
            <option value="zh">ZH</option>
            <option value="ja">JA</option>
        </select>
    )
}
