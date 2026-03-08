import React from 'react';
import { AlertTriangle, RefreshCw, Home } from 'lucide-react';
import { Button } from './ui/button';

interface ErrorBoundaryState {
    hasError: boolean;
    error: Error | null;
}

class ErrorBoundary extends React.Component<React.PropsWithChildren, ErrorBoundaryState> {
    constructor(props: React.PropsWithChildren) {
        super(props);
        this.state = { hasError: false, error: null };
    }

    static getDerivedStateFromError(error: Error): ErrorBoundaryState {
        return { hasError: true, error };
    }

    componentDidCatch(error: Error, info: React.ErrorInfo) {
        console.error('[ErrorBoundary] Uncaught render error:', error, info.componentStack);
    }

    handleReload = () => {
        window.location.reload();
    };

    handleGoHome = () => {
        window.location.href = '/apps';
    };

    render() {
        if (this.state.hasError) {
            return (
                <div className="flex items-center justify-center min-h-screen bg-background px-4">
                    <div className="max-w-md w-full text-center space-y-6">
                        <AlertTriangle className="h-12 w-12 text-destructive mx-auto" />
                        <div className="space-y-2">
                            <h1 className="text-xl font-semibold text-foreground">Something went wrong</h1>
                            <p className="text-sm text-muted-foreground">
                                An unexpected error occurred. This has been logged.
                            </p>
                        </div>
                        {this.state.error?.message && (
                            <pre className="text-xs font-mono text-left bg-muted rounded-lg p-4 max-h-24 overflow-auto border border-border">
                                {this.state.error.message}
                            </pre>
                        )}
                        <div className="flex items-center justify-center gap-3">
                            <Button variant="outline" onClick={this.handleGoHome}>
                                <Home className="h-4 w-4 mr-2" />
                                Go to Dashboard
                            </Button>
                            <Button onClick={this.handleReload}>
                                <RefreshCw className="h-4 w-4 mr-2" />
                                Reload Page
                            </Button>
                        </div>
                        <p className="text-xs text-muted-foreground">
                            If this keeps happening, contact support.
                        </p>
                    </div>
                </div>
            );
        }

        return this.props.children;
    }
}

export default ErrorBoundary;
