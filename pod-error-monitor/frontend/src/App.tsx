import React, { useState, useEffect } from 'react';
import ContextSwitcher from './components/ContextSwitcher';
import useKubeContext from './hooks/useKubeContext';
import './App.css';

interface NamespaceStats {
  name: string;
  totalErrors: number;
  score: number;
  uniquePods: number;
  crashLoop: number;
  imagePull: number;
  highRestarts: number;
  totalRestarts: number;
}

interface PodError {
  namespace: string;
  podName: string;
  errorType: string;
  errorMessage: string;
  containerName: string;
  restartCount: number;
}

function App() {
  const [namespaces, setNamespaces] = useState<NamespaceStats[]>([]);
  const [selectedNamespace, setSelectedNamespace] = useState<string | null>(null);
  const [podErrors, setPodErrors] = useState<PodError[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  const {
    contexts,
    currentContext,
    isLoading: isContextSwitching,
    error: contextError,
    switchContext,
  } = useKubeContext();

  useEffect(() => {
    fetchNamespaces();
    const interval = setInterval(fetchNamespaces, 5000);
    return () => clearInterval(interval);
  }, [currentContext]); // Refetch when context changes

  useEffect(() => {
    if (selectedNamespace) {
      fetchPodErrors(selectedNamespace);
    }
  }, [selectedNamespace, currentContext]); // Refetch when context or namespace changes

  const fetchNamespaces = async () => {
    try {
      const response = await fetch('http://localhost:8080/api/namespaces');
      if (!response.ok) {
        throw new Error('Failed to fetch namespaces');
      }
      const data = await response.json();
      setNamespaces(data);
      setLoading(false);
      setError(null);
    } catch (err) {
      setError('Failed to fetch namespace data');
      setLoading(false);
    }
  };

  const fetchPodErrors = async (namespace: string) => {
    try {
      const response = await fetch(`http://localhost:8080/api/namespaces/${namespace}/pods`);
      if (!response.ok) {
        throw new Error('Failed to fetch pod errors');
      }
      const data = await response.json();
      setPodErrors(data);
      setError(null);
    } catch (err) {
      setError('Failed to fetch pod errors');
    }
  };

  const handleNamespaceClick = (namespace: string) => {
    setSelectedNamespace(selectedNamespace === namespace ? null : namespace);
  };

  const handleContextSwitch = async (context: string) => {
    await switchContext(context);
    // Reset selected namespace when switching contexts
    setSelectedNamespace(null);
    setPodErrors([]);
  };

  if (loading) return <div className="loading">Loading...</div>;
  if (error || contextError) {
    return <div className="error">{error || contextError}</div>;
  }

  return (
    <div className="container mx-auto px-4 py-8">
      <ContextSwitcher
        contexts={contexts}
        currentContext={currentContext}
        onContextSwitch={handleContextSwitch}
        isLoading={isContextSwitching}
      />

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {namespaces.map((ns) => (
          <div
            key={ns.name}
            className={`p-4 rounded-lg shadow cursor-pointer transition-all duration-200 hover:shadow-lg ${
              selectedNamespace === ns.name ? 'ring-2 ring-blue-500' : ''
            }`}
            onClick={() => handleNamespaceClick(ns.name)}
          >
            <h3 className="text-lg font-semibold">{ns.name}</h3>
            <p className="text-sm text-gray-600">{ns.uniquePods} affected pods</p>
            <p className="text-sm text-gray-600">Score: {ns.score.toFixed(1)}</p>
            <div className="mt-2 flex flex-wrap gap-2">
              <span className="inline-block bg-red-100 text-red-800 px-2 py-1 rounded text-xs">
                CrashLoop: {ns.crashLoop}
              </span>
              <span className="inline-block bg-yellow-100 text-yellow-800 px-2 py-1 rounded text-xs">
                ImagePull: {ns.imagePull}
              </span>
              <span className="inline-block bg-orange-100 text-orange-800 px-2 py-1 rounded text-xs">
                High Restarts: {ns.highRestarts}
              </span>
            </div>
          </div>
        ))}
      </div>

      {selectedNamespace && (
        <div className="mt-8">
          <h3 className="text-xl font-bold mb-4">Pod Errors in {selectedNamespace}</h3>
          <div className="space-y-4">
            {podErrors.map((error, index) => (
              <div key={index} className="bg-white p-4 rounded-lg shadow">
                <div className="flex justify-between items-start">
                  <div>
                    <h4 className="font-semibold">{error.podName}</h4>
                    <p className="text-sm text-gray-600">Container: {error.containerName}</p>
                  </div>
                  <span className="text-sm bg-red-100 text-red-800 px-2 py-1 rounded">
                    {error.errorType}
                  </span>
                </div>
                <p className="mt-2 text-sm text-gray-700">{error.errorMessage}</p>
                {error.restartCount > 0 && (
                  <p className="mt-1 text-sm text-gray-600">
                    Restart Count: {error.restartCount}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
