import React from 'react';
import { Menu, Transition } from '@headlessui/react';
import { ChevronDownIcon, CheckIcon } from '@heroicons/react/20/solid';

interface ContextSwitcherProps {
  contexts: string[];
  currentContext: string;
  onContextSwitch: (context: string) => void;
  isLoading: boolean;
}

const ContextSwitcher: React.FC<ContextSwitcherProps> = ({
  contexts,
  currentContext,
  onContextSwitch,
  isLoading,
}) => {
  return (
    <div className="mb-8">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-bold">Kubernetes Context</h2>
        {isLoading && (
          <span className="text-sm text-gray-500">Switching context...</span>
        )}
      </div>
      <div className="relative">
        <Menu>
          {({ open }) => (
            <>
              <Menu.Button
                disabled={isLoading}
                className={`inline-flex w-full justify-between items-center rounded-md bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm ring-1 ring-inset ring-gray-300 hover:bg-gray-50 ${
                  isLoading ? 'opacity-50 cursor-not-allowed' : ''
                }`}
              >
                <span className="flex items-center space-x-2">
                  <span className="block truncate">{currentContext}</span>
                </span>
                <ChevronDownIcon
                  className={`ml-2 h-5 w-5 text-gray-400 transition-transform duration-200 ${
                    open ? 'transform rotate-180' : ''
                  }`}
                  aria-hidden="true"
                />
              </Menu.Button>

              <Transition
                enter="transition duration-100 ease-out"
                enterFrom="transform scale-95 opacity-0"
                enterTo="transform scale-100 opacity-100"
                leave="transition duration-75 ease-out"
                leaveFrom="transform scale-100 opacity-100"
                leaveTo="transform scale-95 opacity-0"
              >
                <Menu.Items className="absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-md bg-white py-1 text-base shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none sm:text-sm">
                  {contexts.map((context) => (
                    <Menu.Item key={context}>
                      {({ active }) => (
                        <button
                          onClick={() => onContextSwitch(context)}
                          className={`${
                            active ? 'bg-blue-50' : ''
                          } ${
                            context === currentContext ? 'bg-blue-50' : ''
                          } group flex w-full items-center px-4 py-2 text-sm text-gray-900 hover:bg-blue-50`}
                        >
                          <span className="flex items-center justify-between w-full">
                            <span className="block truncate">{context}</span>
                            {context === currentContext && (
                              <CheckIcon
                                className="h-4 w-4 text-blue-600"
                                aria-hidden="true"
                              />
                            )}
                          </span>
                        </button>
                      )}
                    </Menu.Item>
                  ))}
                </Menu.Items>
              </Transition>
            </>
          )}
        </Menu>
      </div>
    </div>
  );
};

export default ContextSwitcher; 