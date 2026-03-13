"use client"

import * as React from "react"
import { Moon, Sun } from "lucide-react"
import { useTheme } from "next-themes"
import { useTranslation } from "react-i18next"

import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from "@/components/ui/sidebar"

export function ThemeToggle() {
    const { theme, setTheme } = useTheme()
    const { t } = useTranslation()

    return (
        <SidebarMenu>
            <SidebarMenuItem>
                <SidebarMenuButton onClick={() => setTheme(theme === "dark" ? "light" : "dark")}>
                    <Sun className="h-[1.2rem] w-[1.2rem] rotate-0 scale-100 transition-all dark:-rotate-90 dark:scale-0" />
                    <Moon className="absolute h-[1.2rem] w-[1.2rem] rotate-90 scale-0 transition-all dark:rotate-0 dark:scale-100" />
                    <span>{t("theme.toggle", "Toggle theme")}</span>
                </SidebarMenuButton>
            </SidebarMenuItem>
        </SidebarMenu>
    )
}
