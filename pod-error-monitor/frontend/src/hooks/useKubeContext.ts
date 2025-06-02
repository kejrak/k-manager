import { useState, useEffect, useCallback } from 'react';

interface KubeConfig {
  currentContext: string;
  contexts: string[];
}

interface UseKubeContextResult {
  contexts: string[];
  currentContext: string;
  isLoading: boolean;
  error: string | null;
  switchContext: (context: string) => Promise<void>;
  refreshContexts: () => Promise<void>;
}

const useKubeContext = (): UseKubeContextResult => {
  const [contexts, setContexts] = useState<string[]>([]);
  const [currentContext, setCurrentContext] = useState<string>('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchContexts = async () => {
    try {
      const response = await fetch('http://localhost:8080/api/contexts');
      if (!response.ok) {
        throw new Error('Failed to fetch contexts');
      }
      const data: KubeConfig = await response.json();
      setContexts(data.contexts);
      setCurrentContext(data.currentContext);
      setError(null);
    } catch (err) {
      setError('Failed to fetch Kubernetes contexts');
    }
  };

  const switchContext = async (context: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await fetch(`http://localhost:8080/api/contexts/${context}`, {
        method: 'POST',
      });
      if (!response.ok) {
        throw new Error('Failed to switch context');
      }
      const data: KubeConfig = await response.json();
      setCurrentContext(data.currentContext);
      setError(null);
    } catch (err) {
      setError('Failed to switch Kubernetes context');
    } finally {
      setIsLoading(false);
    }
  };

  const refreshContexts = useCallback(async () => {
    await fetchContexts();
  }, []);

  useEffect(() => {
    fetchContexts();
  }, []);

  return {
    contexts,
    currentContext,
    isLoading,
    error,
    switchContext,
    refreshContexts,
  };
};

export default useKubeContext; 