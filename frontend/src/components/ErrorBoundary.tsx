import { Component, type ErrorInfo, type ReactNode } from "react";
import { reportError } from "../lib/errorReporter";

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
}

function extractComponentName(componentStack: string | null): string | null {
  if (!componentStack) return null;
  const match = componentStack.match(/^\s*at (\w+)/);
  return match ? match[1] : null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    const componentName = extractComponentName(
      info.componentStack ?? null,
    );
    reportError(error, "error_boundary", componentName ?? undefined);
  }

  render(): ReactNode {
    if (this.state.hasError) {
      return (
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            height: "100vh",
            gap: "16px",
            fontFamily: "system-ui, sans-serif",
          }}
        >
          <h1 style={{ fontSize: "1.5rem", margin: 0 }}>
            Something went wrong
          </h1>
          <p style={{ color: "#666", margin: 0 }}>
            An unexpected error occurred. Please try refreshing the page.
          </p>
          <button
            onClick={() => window.location.reload()}
            style={{
              padding: "8px 24px",
              fontSize: "1rem",
              cursor: "pointer",
              border: "1px solid #ccc",
              borderRadius: "4px",
              background: "#fff",
            }}
          >
            Refresh
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
