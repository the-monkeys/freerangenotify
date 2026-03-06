import React from 'react';
import { Inbox } from 'lucide-react';
import { Button } from './ui/button';

interface EmptyStateProps {
    icon?: React.ReactNode;
    title: string;
    description?: string;
    action?: {
        label: string;
        onClick: () => void;
    };
}

const EmptyState: React.FC<EmptyStateProps> = ({
    icon,
    title,
    description,
    action,
}) => {
    return (
        <div className="flex flex-col items-center justify-center py-16 px-4">
            <div className="text-muted-foreground mb-4">
                {icon || <Inbox className="h-12 w-12" />}
            </div>
            <h3 className="text-lg font-medium text-foreground mb-1">{title}</h3>
            {description && (
                <p className="text-sm text-muted-foreground max-w-sm text-center mb-4">
                    {description}
                </p>
            )}
            {action && (
                <Button onClick={action.onClick} size="sm">
                    {action.label}
                </Button>
            )}
        </div>
    );
};

export default EmptyState;
