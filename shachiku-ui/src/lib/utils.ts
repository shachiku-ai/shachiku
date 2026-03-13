import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function getProviderErrorMsg(t: (key: string, defaultMessage: string) => string, error: string): string {
  if (!error) return t("errors.unknown", "Unknown error");
  const lowerError = error.toLowerCase();

  if (lowerError.includes("incorrect api key") || lowerError.includes("invalid api key") || lowerError.includes("authenticationerror") || lowerError.includes("401") || lowerError.includes("unauthorized")) {
    return t("errors.invalidApiKey", "Invalid API Key provided. Please check your credentials.");
  }
  if (lowerError.includes("connection refused") || lowerError.includes("dial tcp") || lowerError.includes("no such host") || lowerError.includes("timeout") || lowerError.includes("network error")) {
    return t("errors.networkError", "Network connection failed. Please check your internet or proxy settings.");
  }
  if (lowerError.includes("api key is required")) {
    return t("errors.apiKeyRequired", "API Key is required to fetch models.");
  }
  if (lowerError.includes("failed to fetch openai models")) {
    return t("errors.openaiError", "Failed to verify OpenAI API Key: ") + error.replace("failed to fetch OpenAI models: ", "");
  }
  if (lowerError.includes("failed to init gemini") || lowerError.includes("failed to fetch gemini")) {
    const parts = error.split(": ");
    return t("errors.geminiError", "Failed to verify Gemini API Key: ") + (parts.length > 1 ? parts.slice(1).join(": ") : error);
  }
  if (lowerError.includes("unsupported provider")) {
    return t("errors.unsupportedProvider", "Unsupported AI provider configured.");
  }

  return error;
}
