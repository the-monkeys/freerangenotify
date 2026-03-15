import React from 'react';
import { Loader2 } from 'lucide-react';
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from './ui/dialog';
import { Button } from './ui/button';

interface ConfirmDeleteDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	title: string;
	description: string;
	confirmLabel?: string;
	cancelLabel?: string;
	confirmVariant?: 'destructive' | 'default';
	loading?: boolean;
	onConfirm: () => void | Promise<void>;
}

const ConfirmDeleteDialog: React.FC<ConfirmDeleteDialogProps> = ({
	open,
	onOpenChange,
	title,
	description,
	confirmLabel = 'Delete',
	cancelLabel = 'Cancel',
	confirmVariant = 'destructive',
	loading = false,
	onConfirm,
}) => {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-md rounded-md border border-border/70 p-5 shadow-lg" showCloseButton={false}>
				<DialogHeader className="gap-1.5">
					<DialogTitle className="text-[15px] font-semibold tracking-tight">{title}</DialogTitle>
					<DialogDescription className="text-sm leading-5 text-muted-foreground/90">{description}</DialogDescription>
				</DialogHeader>
				<DialogFooter
					className="mx-0 mb-0 rounded-none border-0 bg-transparent p-0 pt-4 flex-row justify-end gap-2"
					showCloseButton={false}
				>
					<Button
						type="button"
						variant="outline"
						className="rounded-md"
						onClick={() => onOpenChange(false)}
						disabled={loading}
					>
						{cancelLabel}
					</Button>
					<Button
						type="button"
						variant={confirmVariant === 'destructive' ? 'destructive' : 'default'}
						className="rounded-md"
						onClick={onConfirm}
						disabled={loading}
					>
						{loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
						{confirmLabel}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
};

export default ConfirmDeleteDialog;
