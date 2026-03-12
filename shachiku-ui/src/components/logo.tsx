"use client"

import { useTheme } from "next-themes"
import dynamic from "next/dynamic"
import { useEffect, useState } from "react"
import logoLight from "../../public/logo-light.json"
import logoDark from "../../public/logo-dark.json"

// Lottie must be dynamically imported to avoid SSR issues
const Lottie = dynamic(() => import("lottie-react"), { ssr: false })

export function Logo({ className }: { className?: string }) {
    const { resolvedTheme } = useTheme()
    const [mounted, setMounted] = useState(false)

    useEffect(() => {
        setMounted(true) // wait for hydration to safely access resolvedTheme
    }, [])

    if (!mounted) {
        return <div className={className} />
    }

    const isDark = resolvedTheme === "dark"
    const animationData = isDark ? logoDark : logoLight

    return (
        <Lottie animationData={animationData} loop={true} className={className} />
    )
}
