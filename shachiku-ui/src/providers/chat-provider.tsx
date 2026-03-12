"use client"

import React, { createContext, useContext, useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { API_URL } from "@/lib/api"

export type Message = {
    Role: string
    Content: string
    Thought?: string
}

type ChatContextType = {
    messages: Message[]
    setMessages: React.Dispatch<React.SetStateAction<Message[]>>
    input: string
    setInput: React.Dispatch<React.SetStateAction<string>>
    attachments: File[]
    setAttachments: React.Dispatch<React.SetStateAction<File[]>>
    loading: boolean
    fetching: boolean
    abortController: AbortController | null
    setAbortController: React.Dispatch<React.SetStateAction<AbortController | null>>
    handleSend: (e: React.FormEvent, inputMsg: string, attachs: File[]) => Promise<void>
    executeClearShortMemory: () => Promise<void>
}

const ChatContext = createContext<ChatContextType | undefined>(undefined)

export function ChatProvider({ children }: { children: React.ReactNode }) {
    const { t } = useTranslation()
    const [messages, setMessages] = useState<Message[]>([])
    const [input, setInput] = useState("")
    const [attachments, setAttachments] = useState<File[]>([])
    const [loading, setLoading] = useState(false)
    const [fetching, setFetching] = useState(true)
    const [abortController, setAbortController] = useState<AbortController | null>(null)

    useEffect(() => {
        // Fetch initial chat history from memory
        fetch(`${API_URL}/memory`)
            .then((res) => res.json())
            .then((data) => {
                if (Array.isArray(data)) {
                    setMessages(data)
                }
            })
            .catch((err) => console.error("Failed to load history:", err))
            .finally(() => setFetching(false))
    }, [])

    useEffect(() => {
        const interval = setInterval(() => {
            if (loading) return; // Do not fetch background memory while actively chatting
            fetch(`${API_URL}/memory`)
                .then((res) => res.json())
                .then((data) => {
                    if (Array.isArray(data)) {
                        setMessages(prev => {
                            if (JSON.stringify(prev) === JSON.stringify(data)) return prev;
                            return data;
                        });
                    }
                })
                .catch(console.error);
        }, 3000);
        return () => clearInterval(interval);
    }, [loading]);

    const executeClearShortMemory = async () => {
        try {
            await fetch(`${API_URL}/memory`, { method: "DELETE" })
            setMessages([])
        } catch (err) {
            console.error("Failed to clear short-term memory:", err)
        }
    }

    const handleSend = async (e: React.FormEvent, inputMsg: string, attachs: File[]) => {
        e.preventDefault()
        if (!inputMsg.trim() || loading) return

        setMessages((prev) => [...prev, { Role: "user", Content: inputMsg }, { Role: "agent", Content: "" }])
        setLoading(true)

        const controller = new AbortController()
        setAbortController(controller)

        try {
            let body;
            const headers: Record<string, string> = {};

            if (attachs.length > 0) {
                const formData = new FormData();
                formData.append("message", inputMsg);
                attachs.forEach(file => {
                    formData.append("files", file);
                });
                body = formData;
            } else {
                headers["Content-Type"] = "application/json";
                body = JSON.stringify({ message: inputMsg });
            }

            const res = await fetch(`${API_URL}/chat`, {
                method: "POST",
                headers,
                body,
                signal: controller.signal,
            })

            if (!res.ok) {
                throw new Error(`HTTP error! status: ${res.status}`)
            }

            const reader = res.body?.getReader()
            if (!reader) throw new Error("No reader available")

            const decoder = new TextDecoder("utf-8")
            let done = false
            let buffer = ""
            let currentAgentMsg = ""
            let currentAgentThought = ""

            while (!done) {
                const { value, done: readerDone } = await reader.read()
                done = readerDone
                if (value) {
                    buffer += decoder.decode(value, { stream: true })
                    const lines = buffer.split("\n\n")
                    buffer = lines.pop() || ""

                    for (const chunk of lines) {
                        if (chunk.trim().startsWith("data: ")) {
                            const dataStr = chunk.replace(/^data: /, "").trim()
                            if (!dataStr) continue

                            try {
                                const parsed = JSON.parse(dataStr)
                                if (parsed.type === "step") {
                                    currentAgentThought += (currentAgentThought ? "\n\n" : "") + parsed.content
                                    setMessages((prev) => {
                                        const newMsgs = [...prev]
                                        newMsgs[newMsgs.length - 1] = { Role: "agent", Content: currentAgentMsg, Thought: currentAgentThought }
                                        return newMsgs
                                    })
                                } else if (parsed.type === "result") {
                                    currentAgentMsg = parsed.content
                                    setMessages((prev) => {
                                        const newMsgs = [...prev]
                                        newMsgs[newMsgs.length - 1] = { Role: "agent", Content: currentAgentMsg, Thought: currentAgentThought }
                                        return newMsgs
                                    })
                                } else if (parsed.error) {
                                    setMessages((prev) => [...prev, { Role: "system", Content: `Error: ${parsed.error}` }])
                                }
                            } catch (e) {
                                console.error("Failed to parse stream chunk", e)
                            }
                        }
                    }
                }
            }
        } catch (err: unknown) {
            if (err instanceof Error && err.name === "AbortError") {
                setMessages((prev) => [...prev, { Role: "system", Content: t("chat.stopped", "Conversation stopped.") }])
            } else {
                const errorMessage = err instanceof Error ? err.message : String(err)
                setMessages((prev) => [...prev, { Role: "system", Content: `${t("chat.failed", "Failed to connect to API: ")}${errorMessage}` }])
            }
        } finally {
            setLoading(false)
            setAbortController(null)
        }
    }

    return (
        <ChatContext.Provider value={{
            messages, setMessages,
            input, setInput,
            attachments, setAttachments,
            loading, fetching, abortController, setAbortController,
            handleSend, executeClearShortMemory
        }}>
            {children}
        </ChatContext.Provider>
    )
}

export function useChat() {
    const context = useContext(ChatContext)
    if (context === undefined) {
        throw new Error("useChat must be used within a ChatProvider")
    }
    return context
}
