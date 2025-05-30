import React, { useState, useEffect } from 'react';
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

  useEffect(() => {
    fetchNamespaces();
    const interval = setInterval(fetchNamespaces, 5000); // Refresh every 5 seconds
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (selectedNamespace) {
      fetchPodErrors(selectedNamespace);
    }
  }, [selectedNamespace]);

  const fetchNamespaces = async () => {
    try {
      const response = await fetch('http://localhost:8080/api/namespaces');
      const data = await response.json();
      setNamespaces(data);
      setLoading(false);
    } catch (err) {
      setError('Failed to fetch namespace data');
      setLoading(false);
    }
  };

  const fetchPodErrors = async (namespace: string) => {
    try {
      const response = await fetch(`http://localhost:8080/api/namespaces/${namespace}/pods`);
      const data = await response.json();
      setPodErrors(data);
    } catch (err) {
      setError('Failed to fetch pod errors');
    }
  };

  const handleNamespaceClick = (namespace: string) => {
    setSelectedNamespace(selectedNamespace === namespace ? null : namespace);
  };

  if (loading) return <div className="loading">Loading...</div>;
  if (error) return <div className="error">{error}</div>;

  return (
    <div className="container mx-auto p-4">
      <h1 className="text-3xl font-bold mb-6">Kubernetes Pod Error Monitor</h1>
      
      <div className="grid gap-4">
        {namespaces.map((ns) => (
          <div key={ns.name} className="bg-white rounded-lg shadow-md overflow-hidden">
            <div 
              className={`p-4 cursor-pointer transition-colors ${
                selectedNamespace === ns.name ? 'bg-blue-50' : 'hover:bg-gray-50'
              }`}
              onClick={() => handleNamespaceClick(ns.name)}
            >
              <div className="flex justify-between items-center">
                <div>
                  <h2 className="text-xl font-semibold">{ns.name}</h2>
                  <div className="text-sm text-gray-600">
                    {ns.uniquePods} affected pods • Score: {ns.score.toFixed(1)}
                  </div>
                </div>
                <div className="flex items-center">
                  <div className="bg-red-100 text-red-800 px-3 py-1 rounded-full text-sm font-medium">
                    {ns.totalErrors} {ns.totalErrors === 1 ? 'error' : 'errors'}
                  </div>
                </div>
              </div>
            </div>

            {selectedNamespace === ns.name && (
              <div className="border-t border-gray-200">
                <div className="p-4">
                  <div className="grid grid-cols-4 gap-4 mb-4">
                    <div className="bg-orange-50 p-3 rounded">
                      <div className="text-orange-800 font-medium">CrashLoop</div>
                      <div className="text-2xl font-bold">{ns.crashLoop}</div>
                    </div>
                    <div className="bg-purple-50 p-3 rounded">
                      <div className="text-purple-800 font-medium">Image Pull</div>
                      <div className="text-2xl font-bold">{ns.imagePull}</div>
                    </div>
                    <div className="bg-blue-50 p-3 rounded">
                      <div className="text-blue-800 font-medium">High Restarts</div>
                      <div className="text-2xl font-bold">{ns.highRestarts}</div>
                    </div>
                    <div className="bg-gray-50 p-3 rounded">
                      <div className="text-gray-800 font-medium">Total Restarts</div>
                      <div className="text-2xl font-bold">{ns.totalRestarts}</div>
                    </div>
                  </div>

                  <div className="overflow-x-auto">
                    <table className="min-w-full divide-y divide-gray-200">
                      <thead className="bg-gray-50">
                        <tr>
                          <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Pod</th>
                          <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Container</th>
                          <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Error Type</th>
                          <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Restarts</th>
                          <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Message</th>
                        </tr>
                      </thead>
                      <tbody className="bg-white divide-y divide-gray-200">
                        {podErrors.map((error, index) => (
                          <tr key={`${error.podName}-${index}`}>
                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{error.podName}</td>
                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{error.containerName}</td>
                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{error.errorType}</td>
                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{error.restartCount}</td>
                            <td className="px-6 py-4 text-sm text-gray-500">{error.errorMessage}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

export default App;
