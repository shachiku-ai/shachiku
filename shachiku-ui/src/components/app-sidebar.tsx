"use client"

import * as React from "react"
import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import { useTranslation } from "react-i18next"
import { TFunction } from "i18next"
import { API_URL } from "@/lib/api"

import { NavMain } from "@/components/nav-main"
import { NavSecondary } from "@/components/nav-secondary"
import { Logo } from "@/components/logo"
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { MessageSquareIcon, DatabaseIcon, WrenchIcon, Settings2Icon, ListTodoIcon, ActivityIcon } from "lucide-react"

const getData = (t: TFunction) => ({
  user: {
    name: "Agent Admin",
    email: "admin@agent.local",
    avatar: "/avatars/shadcn.jpg",
  },
  navMain: [
    {
      title: t("navMain.chat", "Chat & History"),
      url: "/",
      icon: <MessageSquareIcon />,
    },
    {
      title: t("navMain.token", "Token Dashboard"),
      url: "/dashboard",
      icon: <ActivityIcon />,
    },
    {
      title: t("navMain.memory", "Memory Management"),
      url: "/memory",
      icon: <DatabaseIcon />,
    },
    {
      title: t("navMain.skills", "Agent Skills"),
      url: "/skills",
      icon: <WrenchIcon />,
    },
    {
      title: t("navMain.tasks", "Agent Tasks"),
      url: "/tasks",
      icon: <ListTodoIcon />,
    },
  ],
  navSecondary: [
    {
      title: t("navSecondary.config", "Config API"),
      url: "/settings",
      icon: <Settings2Icon />,
    },
  ],
})
export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const pathname = usePathname();
  const router = useRouter();
  const { t } = useTranslation();
  const data = React.useMemo(() => getData(t), [t]);

  React.useEffect(() => {
    const checkConfig = async () => {
      try {
        const res = await fetch(`${API_URL}/config`);
        if (res.ok) {
          const cfg = await res.json();
          if (!cfg.setup_completed && pathname !== "/onboarding") {
            router.push("/onboarding");
          }
        }
      } catch (e) {
        console.error("Failed to fetch config", e);
      }
    };
    checkConfig();
  }, [pathname, router]);

  if (pathname === "/onboarding") {
    return null;
  }

  return (
    <Sidebar collapsible="offcanvas" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              className="data-[slot=sidebar-menu-button]:p-1.5! h-15!"
              render={<Link href="/" />}
            >
              <Logo className="size-10!" />
              <span className="text-lg font-semibold">Shachiku</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={data.navMain} />
        <NavSecondary items={data.navSecondary} className="mt-auto" />
      </SidebarContent>
    </Sidebar>
  )
}
