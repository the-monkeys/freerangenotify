import React, { useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { toast } from 'sonner';

/**
 * SSOCallback is the page that handles the redirect back from the backend
 * after a successful OIDC SSO login. It expects access_token and refresh_token
 * in the URL query parameters.
 */
const SSOCallback: React.FC = () => {
    const navigate = useNavigate();
    const location = useLocation();
    const { fetchCurrentUser } = useAuth();

    useEffect(() => {
        const handleCallback = async () => {
            // Parse URL parameters
            const params = new URLSearchParams(location.search);
            const accessToken = params.get('access_token');
            const refreshToken = params.get('refresh_token');
            const error = params.get('error');

            if (error) {
                toast.error(`SSO Login Failed: ${error}`);
                navigate('/login', { replace: true });
                return;
            }

            if (accessToken && refreshToken) {
                // Store tokens
                localStorage.setItem('access_token', accessToken);
                localStorage.setItem('refresh_token', refreshToken);

                try {
                    // Fetch user profile to complete login state
                    if (fetchCurrentUser) {
                        await fetchCurrentUser();
                    } else {
                        // Minimal fallback if fetchCurrentUser isn't exposed yet
                        window.location.href = '/apps';
                        return;
                    }
                    toast.success('Successfully logged in with Monkeys Identity');
                    navigate('/apps', { replace: true });
                } catch (err) {
                    console.error('Failed to fetch user after SSO:', err);
                    toast.error('Failed to complete login');
                    navigate('/login', { replace: true });
                }
            } else {
                toast.error('Invalid callback URL');
                navigate('/login', { replace: true });
            }
        };

        handleCallback();
    }, [location, navigate, fetchCurrentUser]); // fetchCurrentUser is stable (useCallback)

    return (
        <div className="min-h-screen flex items-center justify-center bg-gray-50">
            <div className="text-center">
                <Loader2 className="h-10 w-10 animate-spin text-blue-600 mx-auto mb-4" />
                <h2 className="text-xl font-semibold text-gray-800">Completing login...</h2>
                <p className="text-gray-500 mt-2">Please wait while we log you in.</p>
            </div>
        </div>
    );
};

export default SSOCallback;
