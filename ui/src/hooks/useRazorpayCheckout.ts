import { useState } from 'react';
import { billingAPI } from '../services/api';

// Razorpay SDK Types
interface RazorpayOptions {
    key: string;
    amount: string; // Amount is in currency subunits (Paisa)
    currency: string;
    name: string;
    description: string;
    image?: string;
    order_id: string; // This is a Razorpay Order ID
    handler: (response: RazorpayPaymentResponse) => void;
    prefill?: {
        name?: string;
        email?: string;
        contact?: string;
    };
    notes?: {
        address?: string;
    };
    theme?: {
        color?: string;
    };
    modal?: {
        ondismiss: () => void;
    };
}

interface RazorpayPaymentResponse {
    razorpay_payment_id: string;
    razorpay_order_id: string;
    razorpay_signature: string;
}

interface RazorpayInstance {
    on(event: string, callback: (...args: any[]) => void): void;
    open(): void;
}

declare global {
    interface Window {
        Razorpay: {
            new(options: RazorpayOptions): RazorpayInstance;
        };
    }
}

// Ensure the Razorpay script is loaded dynamically
export const loadRazorpayScript = (): Promise<boolean> => {
    return new Promise((resolve) => {
        if (window.Razorpay) {
            resolve(true);
            return;
        }
        const script = document.createElement('script');
        script.src = 'https://checkout.razorpay.com/v1/checkout.js';
        script.onload = () => resolve(true);
        script.onerror = () => resolve(false);
        document.body.appendChild(script);
    });
};

export const useRazorpayCheckout = (onSuccess?: () => void) => {
    const [isCheckoutLoading, setIsCheckoutLoading] = useState(false);

    const initiateCheckout = async (tier: string = 'pro') => {
        setIsCheckoutLoading(true);

        try {
            // Ensure script is loaded
            const loaded = await loadRazorpayScript();
            if (!loaded) {
                alert('Could not load Razorpay SDK. Please check your network.');
                setIsCheckoutLoading(false);
                return;
            }

            // Step 1: Create Order locally
            const orderRes = await billingAPI.checkoutBilling(tier);

            if (orderRes.checkout_url === 'mock_success') {
                alert('Mock payment provider is active. Set FREERANGE_PAYMENT_PROVIDER=razorpay and configure Razorpay keys to collect real card details.');
                setIsCheckoutLoading(false);
                return;
            }

            const orderData = orderRes;

            // Step 2: Configure Razorpay Checkout
            const options: RazorpayOptions = {
                key: orderData.key_id,
                amount: orderData.amount.toString(),
                currency: orderData.currency,
                name: 'FreeRangeNotify',
                description: `Upgrade to ${tier.toUpperCase()}`,
                order_id: orderData.order_id,
                handler: async function (response: RazorpayPaymentResponse) {
                    try {
                        // Step 3: Verify the payment signature on the backend
                        await billingAPI.verifyPayment({
                            razorpay_order_id: response.razorpay_order_id,
                            razorpay_payment_id: response.razorpay_payment_id,
                            razorpay_signature: response.razorpay_signature,
                        });

                        alert(`Payment Successful! You are now on the ${tier.toUpperCase()} plan.`);
                        onSuccess?.();
                    } catch (err: any) {
                        alert(err.response?.data?.error || 'Payment verification failed.');
                    } finally {
                        setIsCheckoutLoading(false);
                    }
                },
                theme: {
                    color: '#0f172a', // Set to slate-900 to match shadcn
                },
                modal: {
                    ondismiss: function () {
                        setIsCheckoutLoading(false);
                        console.log('Checkout Cancelled: Payment was not completed.');
                    }
                }
            };

            const rzp = new window.Razorpay(options);

            rzp.on('payment.failed', function (response: any) {
                alert(response.error.description || 'The payment failed.');
                setIsCheckoutLoading(false);
            });

            rzp.open();

        } catch (error: any) {
            alert(error.response?.data?.error || 'Failed to initialize checkout.');
            setIsCheckoutLoading(false);
        }
    };

    return { initiateCheckout, isCheckoutLoading };
};
